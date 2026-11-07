package importer

import (
	"bufio"
	"bytes"
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/safefetch"
)

const (
	robotsPath         = "/robots.txt"
	robotsMaximumBytes = 512 << 10
	robotsTimeout      = 5 * time.Second
	robotsAgent        = "keeps"
	wildcardAgent      = "*"
)

// robotsAllow applies the site's robots.txt rules (RFC 9309) for the Keeps
// agent, falling back to the "*" group. Robots rules are an access courtesy,
// not authorisation; they never grant access to anything private.
// ponytail: a missing or unreachable robots.txt allows the fetch; follow the RFC's "assume disallow on 5xx" if sites complain.
func (s *Service) robotsAllow(ctx context.Context, pageURL string) (bool, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return false, err
	}
	limits := safefetch.DefaultPageLimits()
	limits.MaxBodyBytes = robotsMaximumBytes
	limits.TotalTimeout = robotsTimeout
	response, err := s.fetcher.Get(ctx, parsed.Scheme+"://"+parsed.Host+robotsPath, limits, "text/plain")
	if err != nil {
		return true, nil
	}
	path := parsed.EscapedPath()
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return robotsPermits(response.Body, path), nil
}

type robotsRule struct {
	allow  bool
	prefix string
}

// robotsPermits picks the most specific matching rule; Allow wins ties.
func robotsPermits(content []byte, path string) bool {
	groups := map[string][]robotsRule{}
	var currentAgents []string
	collectingAgents := false
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		field, value = strings.ToLower(strings.TrimSpace(field)), strings.TrimSpace(value)
		switch field {
		case "user-agent":
			if !collectingAgents {
				currentAgents = nil
			}
			currentAgents = append(currentAgents, strings.ToLower(value))
			collectingAgents = true
		case "allow", "disallow":
			collectingAgents = false
			if value == "" && field == "disallow" {
				continue
			}
			for _, agent := range currentAgents {
				groups[agent] = append(groups[agent], robotsRule{allow: field == "allow", prefix: value})
			}
		}
	}
	rules, specific := groups[robotsAgent]
	if !specific {
		rules = groups[wildcardAgent]
	}
	best := robotsRule{allow: true}
	bestLength := -1
	for _, rule := range rules {
		if !matchesRobots(rule.prefix, path) {
			continue
		}
		longer := len(rule.prefix) > bestLength
		tieAllows := len(rule.prefix) == bestLength && rule.allow
		if longer || tieAllows {
			best, bestLength = rule, len(rule.prefix)
		}
	}
	return best.allow
}

// matchesRobots supports the RFC's "*" wildcard and "$" end anchor.
func matchesRobots(pattern string, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	expression := "^" + strings.ReplaceAll(regexp.QuoteMeta(strings.TrimSuffix(pattern, "$")), `\*`, ".*")
	if anchored {
		expression += "$"
	}
	matched, err := regexp.MatchString(expression, path)
	return err == nil && matched
}
