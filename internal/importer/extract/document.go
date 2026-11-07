package extract

import (
	"strings"

	"golang.org/x/net/html"
)

// walkNodes visits the descendants of root in document order without
// recursion, so deeply nested hostile markup cannot exhaust the stack.
// Returning false from visit skips that node's children.
func walkNodes(root *html.Node, visit func(*html.Node) bool) {
	var stack []*html.Node
	pushChildren := func(parent *html.Node) {
		for child := parent.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	pushChildren(root)
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visit(node) {
			pushChildren(node)
		}
	}
}

func walkElements(root *html.Node, visit func(*html.Node) bool) {
	walkNodes(root, func(node *html.Node) bool {
		if node.Type != html.ElementNode {
			return true
		}
		return visit(node)
	})
}

func attribute(node *html.Node, key string) string {
	for _, nodeAttribute := range node.Attr {
		if nodeAttribute.Namespace == "" && strings.EqualFold(nodeAttribute.Key, key) {
			return nodeAttribute.Val
		}
	}
	return ""
}

func hasAttribute(node *html.Node, key string) bool {
	for _, nodeAttribute := range node.Attr {
		if nodeAttribute.Namespace == "" && strings.EqualFold(nodeAttribute.Key, key) {
			return true
		}
	}
	return false
}
