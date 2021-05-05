package filter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
)

// Filter will compare metrics names and tags against regex
type Filter interface {
	Match(name string, tags map[string]string) bool
	IsHistogramMetric(name string) bool
}

// TagFilter is used to deifne tag filters
type TagFilter map[string][]string

// Decode env variables into TagFilter type
func (f *TagFilter) Decode(filters string) error {
	r := regexp.MustCompile(`:\w`)
	if r.MatchString(filters) {
		return fmt.Errorf("bad format... 'tagName:[regex]' or 'tagName:[regex, regex1, ... regexX]'")
	}

	r = regexp.MustCompile(`(\w*):\[([^\]]*)\]`)
	(*f) = make(map[string][]string)
	matches := r.FindAllStringSubmatch(filters, -1) // matches is [][]string
	for _, match := range matches {
		(*f)[match[1]] = strings.Split(match[2], ",")
	}
	return nil
}

//Filters holds metrics white and black list filters
type Filters struct {
	MetricsBlackList []string
	MetricsWhiteList []string
	MetricsToHisList []string

	MetricsTagBlackList TagFilter
	MetricsTagWhiteList TagFilter

	TagInclude []string
	TagExclude []string
}

type globFilter struct {
	metricWhitelist    glob.Glob
	metricBlacklist    glob.Glob
	metricsToHisList   glob.Glob
	metricTagWhitelist map[string]glob.Glob
	metricTagBlacklist map[string]glob.Glob
	tagInclude         glob.Glob
	tagExclude         glob.Glob
}

// NewGlobFilter grate a new Filter using gobwas/glob as regex engine
func NewGlobFilter(cfg *Filters) Filter {
	cfg.TagInclude = cleanUp(cfg.TagInclude)
	cfg.TagExclude = cleanUp(cfg.TagExclude)

	utils.Logger.Printf("filters: MetricsWhiteList = '%v", cfg.MetricsWhiteList)
	utils.Logger.Printf("filters: MetricsBlackList = '%v", cfg.MetricsBlackList)
	utils.Logger.Printf("filters: MetricsToHisList = '%v", cfg.MetricsToHisList)
	utils.Logger.Printf("filters: MetricsTagWhiteList = '%v", cfg.MetricsTagWhiteList)
	utils.Logger.Printf("filters: MetricsTagBlackList = '%v", cfg.MetricsTagBlackList)
	utils.Logger.Printf("filters: TagInclude = '%v", cfg.TagInclude)
	utils.Logger.Printf("filters: TagExclude = '%v", cfg.TagExclude)

	return &globFilter{
		metricWhitelist:    compile(cfg.MetricsWhiteList),
		metricBlacklist:    compile(cfg.MetricsBlackList),
		metricsToHisList:   compile(cfg.MetricsToHisList),
		metricTagWhitelist: multiCompile(cfg.MetricsTagWhiteList),
		metricTagBlacklist: multiCompile(cfg.MetricsTagBlackList),
		tagInclude:         compile(cfg.TagInclude),
		tagExclude:         compile(cfg.TagExclude),
	}
}

func compile(filters []string) glob.Glob {
	filters = cleanUp(filters)
	if len(filters) == 0 {
		return nil
	}
	if len(filters) == 1 {
		g, _ := glob.Compile(filters[0])
		return g
	}
	g, _ := glob.Compile("{" + strings.Join(filters, ",") + "}")
	return g
}

func multiCompile(filters map[string][]string) map[string]glob.Glob {
	if len(filters) == 0 {
		return nil
	}
	globs := make(map[string]glob.Glob, len(filters))
	for k, v := range filters {
		g := compile(v)
		if g != nil {
			globs[k] = g
		}
	}
	return globs
}

func (gf *globFilter) Match(name string, tags map[string]string) bool {
	if gf.metricWhitelist != nil && !gf.metricWhitelist.Match(name) {
		return false
	}
	if gf.metricBlacklist != nil && gf.metricBlacklist.Match(name) {
		return false
	}

	if gf.metricTagWhitelist != nil && !matchesTags(gf.metricTagWhitelist, tags) {
		return false
	}
	if gf.metricTagBlacklist != nil && matchesTags(gf.metricTagBlacklist, tags) {
		return false
	}

	if gf.tagInclude != nil {
		deleteTags(gf.tagInclude, tags, true)
	}
	if gf.tagExclude != nil {
		deleteTags(gf.tagExclude, tags, false)
	}
	return true
}

func (gf *globFilter) IsHistogramMetric(name string) bool {
	if gf.metricsToHisList != nil && gf.metricsToHisList.Match(name) {
		return true
	}
	return false
}

func matchesTags(matchers map[string]glob.Glob, tags map[string]string) bool {
	for k, matcher := range matchers {
		if val, ok := tags[k]; ok {
			if matcher.Match(val) {
				return true
			}
		}
	}
	return false
}

func deleteTags(matcher glob.Glob, tags map[string]string, include bool) {
	for k := range tags {
		matches := matcher.Match(k)
		if (include && !matches) || (!include && matches) {
			delete(tags, k)
		}
	}
}

func cleanUp(ss []string) (ret []string) {
	for _, s := range ss {
		if len(s) > 0 {
			ret = append(ret, strings.Trim(s, " "))
		}
	}
	return
}
