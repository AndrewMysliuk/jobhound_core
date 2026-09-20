package schema

import "strings"

// Q-list search strings (fixed order) for SlotSearchFetcher-backed sources.
const (
	QueryVue         = "vue"
	QueryFrontend    = "frontend"
	QueryFullStack   = "full-stack"
	QueryTypescript  = "typescript"
	QueryReact       = "react"
	QueryGolang      = "golang"
	QueryNode        = "node"
	QueryAINative    = "AI native"
	QueryAIEngineer  = "AI engineer"
	QueryLLM         = "LLM"
)

// Wellfound /role/r/{slug} segments (fixed order).
const (
	WellfoundSlugFrontendEngineer              = "frontend-engineer"
	WellfoundSlugFullStackEngineer             = "full-stack-engineer"
	WellfoundSlugBackendEngineer               = "backend-engineer"
	WellfoundSlugArtificialIntelligenceEngineer = "artificial-intelligence-engineer"
)

var qList = []string{
	QueryVue,
	QueryFrontend,
	QueryFullStack,
	QueryTypescript,
	QueryReact,
	QueryGolang,
	QueryNode,
	QueryAINative,
	QueryAIEngineer,
	QueryLLM,
}

var wellfoundSlugs = []string{
	WellfoundSlugFrontendEngineer,
	WellfoundSlugFullStackEngineer,
	WellfoundSlugBackendEngineer,
	WellfoundSlugArtificialIntelligenceEngineer,
}

// Queries returns matrix queries for sourceID. Nil or empty means catalog (one Fetch child, no keyword/slug).
func Queries(sourceID string) []string {
	switch strings.ToLower(strings.TrimSpace(sourceID)) {
	case "europe_remotely", "working_nomads", "himalayas", "builtin", "remotify_europe":
		return qList
	case "wellfound":
		return wellfoundSlugs
	case "we_work_remotely", "vue_jobs", "golang_cafe":
		return nil
	default:
		return nil
	}
}
