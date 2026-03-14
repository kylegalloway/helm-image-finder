package main

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// ImageEntry holds a single discovered container image along with the
// Kubernetes resource and container that reference it.
type ImageEntry struct {
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	ResourceName  string `json:"resourceName"`
	ChartLabel    string `json:"chartLabel"`
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	Tag           string `json:"tag"`
}

// kubernetesResourceHeader is used only to unmarshal the top-level kind and
// metadata fields from a document. The full node tree is walked separately.
type kubernetesResourceHeader struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
}

// extractImagesFromRenderedChart parses a multi-document YAML string (as
// produced by `helm template`) and returns one ImageEntry per container image
// found anywhere in the document tree.
func extractImagesFromRenderedChart(renderedYAML string) ([]ImageEntry, error) {
	var allImageEntries []ImageEntry

	decoder := yaml.NewDecoder(strings.NewReader(renderedYAML))

	for {
		var documentNode yaml.Node
		err := decoder.Decode(&documentNode)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error decoding YAML document: %w", err)
		}

		// A null or empty document (e.g. a bare `---`) has no content.
		if documentNode.Kind == 0 || len(documentNode.Content) == 0 {
			continue
		}

		// Decode into a lightweight struct just to read kind + metadata.
		var header kubernetesResourceHeader
		if err := documentNode.Decode(&header); err != nil || header.Kind == "" {
			continue // not a Kubernetes resource, skip
		}

		namespace := header.Metadata.Namespace
		if namespace == "" {
			namespace = "default"
		}

		// The helm.sh/chart label is stamped by Helm on every rendered
		// resource and contains the chart name + version (e.g. "redis-17.3.1").
		// For umbrella charts this identifies which subchart owns the resource.
		chartLabel := header.Metadata.Labels["helm.sh/chart"]

		// Walk the full node tree for image fields, starting at the root
		// mapping inside the document wrapper node.
		rootMappingNode := documentNode.Content[0]
		entriesFromDocument := walkNodeTreeForImages(
			rootMappingNode,
			header.Kind,
			namespace,
			header.Metadata.Name,
			chartLabel,
			"",
		)

		allImageEntries = append(allImageEntries, entriesFromDocument...)
	}

	return allImageEntries, nil
}

// walkNodeTreeForImages recursively descends a yaml.Node tree and collects
// every mapping node that contains an "image" key. The containerName
// parameter carries the nearest "name" value found while descending, so that
// each image can be attributed to its owning container.
func walkNodeTreeForImages(
	node *yaml.Node,
	kind string,
	namespace string,
	resourceName string,
	chartLabel string,
	containerName string,
) []ImageEntry {
	if node == nil {
		return nil
	}

	var foundEntries []ImageEntry

	if node.Kind == yaml.MappingNode {
		keyValuePairs := buildKeyValueLookup(node)

		// If this mapping has a "name" key, we are likely inside a container
		// or init-container spec. Update the name for any images found here
		// or deeper in the tree.
		if nameNode, hasName := keyValuePairs["name"]; hasName && nameNode.Value != "" {
			containerName = nameNode.Value
		}

		if imageNode, hasImage := keyValuePairs["image"]; hasImage && imageNode.Value != "" {
			imageName, imageTag := splitImageReference(imageNode.Value)
			foundEntries = append(foundEntries, ImageEntry{
				Kind:          kind,
				Namespace:     namespace,
				ResourceName:  resourceName,
				ChartLabel:    chartLabel,
				ContainerName: containerName,
				Image:         imageName,
				Tag:           imageTag,
			})
		}

		// Recurse into every value node (odd indices in Content).
		for index := 1; index < len(node.Content); index += 2 {
			childEntries := walkNodeTreeForImages(
				node.Content[index],
				kind, namespace, resourceName, chartLabel, containerName,
			)
			foundEntries = append(foundEntries, childEntries...)
		}
	} else {
		// Sequence nodes and scalar nodes: recurse into children if any.
		for _, childNode := range node.Content {
			childEntries := walkNodeTreeForImages(
				childNode,
				kind, namespace, resourceName, chartLabel, containerName,
			)
			foundEntries = append(foundEntries, childEntries...)
		}
	}

	return foundEntries
}

// buildKeyValueLookup converts a MappingNode's flat Content slice into a map
// from key string to value node, avoiding repetitive index arithmetic at call
// sites. MappingNode.Content is always [key0, value0, key1, value1, ...].
func buildKeyValueLookup(mappingNode *yaml.Node) map[string]*yaml.Node {
	lookup := make(map[string]*yaml.Node, len(mappingNode.Content)/2)
	for index := 0; index+1 < len(mappingNode.Content); index += 2 {
		keyNode := mappingNode.Content[index]
		valueNode := mappingNode.Content[index+1]
		lookup[keyNode.Value] = valueNode
	}
	return lookup
}

// splitImageReference splits a full image reference into the image name and
// tag (or digest). Examples:
//
//	"nginx:1.25.3"                    → ("nginx", "1.25.3")
//	"registry.io/org/app:v1.0"        → ("registry.io/org/app", "v1.0")
//	"nginx@sha256:abc123"             → ("nginx", "sha256:abc123")
//	"nginx"                           → ("nginx", "latest")
func splitImageReference(fullImageReference string) (imageName string, imageTag string) {
	// Digest references use "@" as the separator (e.g. "nginx@sha256:abc123").
	if nameBeforeDigest, digest, isDigestReference := strings.Cut(fullImageReference, "@"); isDigestReference {
		return nameBeforeDigest, digest
	}

	// Find the last "/" to isolate the final path segment. A colon before
	// this point belongs to a registry host:port, not a tag separator.
	lastSlashIndex := strings.LastIndex(fullImageReference, "/")
	finalPathSegment := fullImageReference[lastSlashIndex+1:]

	if imageWithoutTag, tag, hasTag := strings.Cut(finalPathSegment, ":"); hasTag {
		prefix := fullImageReference[:lastSlashIndex+1]
		return prefix + imageWithoutTag, tag
	}

	return fullImageReference, "latest"
}

// deduplicateByImageAndTag returns a new slice with duplicate image+tag pairs
// removed. When duplicates exist, the first occurrence is kept so that
// resource context is preserved in the output.
func deduplicateByImageAndTag(entries []ImageEntry) []ImageEntry {
	seenImageTags := make(map[string]bool, len(entries))
	uniqueEntries := make([]ImageEntry, 0, len(entries))

	for _, entry := range entries {
		imageTagKey := entry.Image + ":" + entry.Tag
		if !seenImageTags[imageTagKey] {
			seenImageTags[imageTagKey] = true
			uniqueEntries = append(uniqueEntries, entry)
		}
	}

	return uniqueEntries
}
