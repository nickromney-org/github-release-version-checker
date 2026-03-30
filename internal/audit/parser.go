package audit

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func ParseWorkflow(repo Repository, workflowPath string, content []byte) ([]Occurrence, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", workflowPath, err)
	}

	if len(root.Content) == 0 {
		return nil, nil
	}

	doc := root.Content[0]
	jobsNode := mappingValue(doc, "jobs")
	if jobsNode == nil || jobsNode.Kind != yaml.MappingNode {
		return nil, nil
	}

	var occurrences []Occurrence
	for i := 0; i < len(jobsNode.Content)-1; i += 2 {
		jobNameNode := jobsNode.Content[i]
		jobNode := jobsNode.Content[i+1]
		jobName := jobNameNode.Value

		if jobUsesNode := mappingValue(jobNode, "uses"); jobUsesNode != nil && jobUsesNode.Kind == yaml.ScalarNode {
			occurrences = append(occurrences, newOccurrence(repo, workflowPath, jobName, "", KindReusableWorkflow, jobUsesNode.Value, jobUsesNode.Line))
		}

		if stepsNode := mappingValue(jobNode, "steps"); stepsNode != nil && stepsNode.Kind == yaml.SequenceNode {
			for _, stepNode := range stepsNode.Content {
				if stepNode.Kind != yaml.MappingNode {
					continue
				}
				usesNode := mappingValue(stepNode, "uses")
				if usesNode == nil || usesNode.Kind != yaml.ScalarNode {
					continue
				}

				stepName := ""
				if nameNode := mappingValue(stepNode, "name"); nameNode != nil && nameNode.Kind == yaml.ScalarNode {
					stepName = nameNode.Value
				}
				if stepName == "" {
					stepName = usesNode.Value
				}

				kind := KindAction
				if isDockerReference(usesNode.Value) {
					kind = KindDockerAction
				}
				occurrences = append(occurrences, newOccurrence(repo, workflowPath, jobName, stepName, kind, usesNode.Value, usesNode.Line))
			}
		}

		if containerNode := mappingValue(jobNode, "container"); containerNode != nil {
			if imageNode := imageValue(containerNode); imageNode != nil && imageNode.Kind == yaml.ScalarNode {
				occurrences = append(occurrences, newOccurrence(repo, workflowPath, jobName, "container", KindContainer, imageNode.Value, imageNode.Line))
			}
		}

		if servicesNode := mappingValue(jobNode, "services"); servicesNode != nil && servicesNode.Kind == yaml.MappingNode {
			for j := 0; j < len(servicesNode.Content)-1; j += 2 {
				serviceName := servicesNode.Content[j].Value
				serviceNode := servicesNode.Content[j+1]
				if imageNode := imageValue(serviceNode); imageNode != nil && imageNode.Kind == yaml.ScalarNode {
					occurrences = append(occurrences, newOccurrence(repo, workflowPath, jobName, serviceName, KindContainer, imageNode.Value, imageNode.Line))
				}
			}
		}
	}

	return occurrences, nil
}

func newOccurrence(repo Repository, workflowPath, job, step string, kind Kind, raw string, line int) Occurrence {
	name, ref, refType, risk := classifyReference(kind, raw)
	occurrence := Occurrence{
		Kind:               kind,
		Name:               name,
		Ref:                ref,
		RefType:            refType,
		Risk:               risk,
		RepoFullName:       repo.FullName,
		RepoPath:           repo.Path,
		RepoDefaultBranch:  repo.DefaultBranch,
		RepoName:           repo.Name,
		RepoPathOrFullName: repo.PathOrFullName(),
		WorkflowPath:       workflowPath,
		Job:                job,
		Step:               step,
		Line:               line,
	}
	if pinning, ok := pinningForOccurrence(occurrence); ok {
		occurrence.Pinning = pinning
	}
	return occurrence
}

func isDockerReference(value string) bool {
	return len(value) > len("docker://") && value[:len("docker://")] == "docker://"
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func imageValue(node *yaml.Node) *yaml.Node {
	switch node.Kind {
	case yaml.ScalarNode:
		return node
	case yaml.MappingNode:
		return mappingValue(node, "image")
	default:
		return nil
	}
}
