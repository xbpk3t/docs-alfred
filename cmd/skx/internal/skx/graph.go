package skx

import (
	"path/filepath"
	"sort"
	"strings"
)

// Edge is a directed dependency between two prompts.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Mode string `json:"mode"` // serial | parallel
}

// Graph is the JSON shape emitted by `skx graph`.
type Graph struct {
	Nodes  []string   `json:"nodes"`
	Edges  []Edge     `json:"edges"`
	Cycles [][]string `json:"cycles"`
}

// BuildGraph extracts the dependency graph from every prompt under dir.
// Edges come only from frontmatter.pl-serial (mode serial) and
// frontmatter.pl-parallel (mode parallel). The legacy pipeline key is not a
// dependency source. Cycles are detected across all edges regardless of mode.
// Files that cannot be parsed are skipped and reported in skipped so the
// graph still covers everything parseable (degrade with reason, never a
// silent partial result).
func BuildGraph(dir string) (*Graph, []string, error) {
	files, err := CollectYML(dir)
	if err != nil {
		return nil, nil, err
	}

	nodeSet := map[string]bool{}
	var edges []Edge
	var skipped []string
	var dangling []string
	for _, f := range files {
		rel, relErr := filepath.Rel(dir, f)
		if relErr == nil && skipHidden(rel) {
			continue // hidden files are cross-ref targets, not graph nodes
		}
		p, err := LoadPrompt(f)
		if err != nil {
			skipped = append(skipped, f)
			continue
		}
		if p.Name != "" {
			nodeSet[p.Name] = true
		}
		fm, ok := getMap(p.Doc, keyFrontmatter)
		if !ok {
			continue
		}
		for _, to := range strList(fm, keyPlSerial) {
			edges = append(edges, Edge{From: p.Name, To: to, Mode: "serial"})
		}
		for _, to := range strList(fm, keyPlParallel) {
			edges = append(edges, Edge{From: p.Name, To: to, Mode: "parallel"})
		}
	}

	// Drop edges that reference a name with no source yml (dangling), so the
	// JSON stays internally consistent. The underlying data issue is reported
	// by `skx check`; dangling names are surfaced here for visibility.
	filtered := edges[:0]
	for _, e := range edges {
		if !nodeSet[e.To] {
			dangling = append(dangling, e.From+"→"+e.To+"("+e.Mode+")")
			continue
		}
		filtered = append(filtered, e)
	}
	edges = filtered

	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	edges = dedupEdges(edges)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Mode < edges[j].Mode
	})

	g := &Graph{Nodes: nodes, Edges: edges}
	g.Cycles = findCycles(nodeSet, edges)
	return g, append(skipped, dangling...), nil
}

// strList reads pl-serial / pl-parallel, accepting a scalar string or a list.
func strList(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if s, isString := v.(string); isString {
		return []string{s}
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, isString := item.(string); isString {
			out = append(out, s)
		}
	}
	return out
}

func dedupEdges(edges []Edge) []Edge {
	seen := map[string]bool{}
	out := make([]Edge, 0, len(edges))
	for _, e := range edges {
		k := e.From + "\x00" + e.To + "\x00" + e.Mode
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// findCycles returns every directed cycle in the graph, each as a path that
// starts and ends on the same node (e.g. [a, b, a]).
func findCycles(nodeSet map[string]bool, edges []Edge) [][]string {
	adj := map[string][]string{}
	for _, e := range edges {
		if nodeSet[e.From] && nodeSet[e.To] {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}
	for _, tos := range adj {
		sort.Strings(tos)
	}

	// 0 = unvisited, 1 = on the current DFS stack, 2 = fully explored.
	state := map[string]int{}
	var stack []string
	var cycles [][]string

	var dfs func(string)
	dfs = func(u string) {
		state[u] = 1
		stack = append(stack, u)
		for _, v := range adj[u] {
			switch state[v] {
			case 1:
				idx := indexOf(stack, v)
				cyc := append([]string{}, stack[idx:]...)
				cyc = append(cyc, v)
				cycles = append(cycles, cyc)
			case 0:
				dfs(v)
			}
		}
		stack = stack[:len(stack)-1]
		state[u] = 2
	}

	for _, n := range sortedKeys(adj) {
		if state[n] == 0 {
			dfs(n)
		}
	}

	return dedupCycles(cycles)
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dedupCycles keeps one rotation of each distinct cycle.
func dedupCycles(cycles [][]string) [][]string {
	seen := map[string]bool{}
	out := make([][]string, 0, len(cycles))
	for _, c := range cycles {
		key := canonicalCycle(c)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		li, lj := len(out[i]), len(out[j])
		if li != lj {
			return li < lj
		}
		for k := 0; k < li; k++ {
			if out[i][k] != out[j][k] {
				return out[i][k] < out[j][k]
			}
		}
		return false
	})
	return out
}

// canonicalCycle rotates a cycle so it starts at its lexicographically
// smallest node, then joins it with a separator for dedup.
func canonicalCycle(c []string) string {
	if len(c) < 2 {
		return strings.Join(c, "\x00")
	}
	body := c[:len(c)-1] // drop the repeated closing node
	start := 0
	for i := 1; i < len(body); i++ {
		if body[i] < body[start] {
			start = i
		}
	}
	rotated := append(append([]string{}, body[start:]...), body[:start]...)
	return strings.Join(append(rotated, body[start]), "\x00")
}
