package analysis

import (
	"sort"

	"skia.googlesource.com/buildbot.git/go/util"
)

// ParamIndex maps parameter key/value pairs to unique trace ids.
type ParamIndex map[string]map[string][]int

// TODO (stephana): This needs to be folded into analysis.go and
// the different loops that iterate over the entire tile need to be
// consolidated into one loop that call the various functions to calculate
// counts, aggregates etc.
// Each group of functions should be in it's own source file (similar to
// triage.go) with analysis.go being the main file of that package.

// getQueryIndex takes the labeled tile and generates an index to look up the
// traces via parameter values. It returns the parameter index, a mapping
// of ids to traces and a map of all parameters and their values.
func getQueryIndex(labeledTile *LabeledTile) (ParamIndex, map[int]*LabeledTrace, map[string][]string) {
	index := map[string]map[string][]int{}
	traceMap := map[int]*LabeledTrace{}

	for _, testTraces := range labeledTile.Traces {
		for _, oneTrace := range testTraces {
			traceMap[oneTrace.Id] = oneTrace
			for k, v := range oneTrace.Params {
				if _, ok := index[k]; !ok {
					index[k] = map[string][]int{}
				}
				if _, ok := index[k][v]; !ok {
					index[k][v] = []int{}
				}
				index[k][v] = append(index[k][v], oneTrace.Id)
			}
		}
	}

	allParams := make(map[string][]string, len(index))
	for param, values := range index {
		allParams[param] = make([]string, 0, len(values))
		for k := range values {
			allParams[param] = append(allParams[param], k)
		}
		sort.Strings(allParams[param])
	}

	return index, traceMap, allParams
}

// queryTraces find all traces that match the given query which contains a
// set of parameters and specific values.
func (a *Analyzer) queryTraces(query map[string][]string) []*LabeledTrace {
	resultSets := make([]map[int]bool, len(query))

	idx, minIdx, minLen := 0, 0, 0

	for key, values := range query {
		if paramMap, ok := a.index[key]; ok {
			resultSets[idx] = map[int]bool{}
			for _, v := range values {
				if indexList, ok := paramMap[v]; ok {
					for _, labelId := range indexList {
						resultSets[idx][labelId] = true
					}
				}
			}
			if (len(resultSets[idx]) < minLen) || (minLen == 0) {
				minLen = len(resultSets)
			}
		}
		idx++
	}

	resultIds := util.IntersectIntSets(resultSets, minIdx)
	result := make([]*LabeledTrace, 0, len(resultIds))
	for _, id := range resultIds {
		if lt, ok := a.traceMap[id]; ok {
			result = append(result, lt)
		}
	}
	return result
}