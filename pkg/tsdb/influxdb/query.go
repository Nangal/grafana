package influxdb

import (
	"fmt"
	"strconv"
	"strings"

	"regexp"

	"github.com/grafana/grafana/pkg/tsdb"
)

var (
	regexpOperatorPattern *regexp.Regexp = regexp.MustCompile(`^\/.*\/$`)
)

func (query *Query) Build(queryContext *tsdb.QueryContext) (string, error) {
	if query.UseRawQuery && query.RawQuery != "" {
		q := query.RawQuery

		q = strings.Replace(q, "$timeFilter", query.renderTimeFilter(queryContext), 1)
		q = strings.Replace(q, "$interval", tsdb.CalculateInterval(queryContext.TimeRange), 1)

		return q, nil
	}

	res := query.renderSelectors(queryContext)
	res += query.renderMeasurement()
	res += query.renderWhereClause()
	res += query.renderTimeFilter(queryContext)
	res += query.renderGroupBy(queryContext)

	return res, nil
}

func (query *Query) renderTags() []string {
	var res []string
	for i, tag := range query.Tags {
		str := ""

		if i > 0 {
			if tag.Condition == "" {
				str += "AND"
			} else {
				str += tag.Condition
			}
			str += " "
		}

		//If the operator is missing we fall back to sensible defaults
		if tag.Operator == "" {
			if regexpOperatorPattern.Match([]byte(tag.Value)) {
				tag.Operator = "=~"
			} else {
				tag.Operator = "="
			}
		}

		textValue := ""
		numericValue, err := strconv.ParseFloat(tag.Value, 64)

		// quote value unless regex or number
		if tag.Operator == "=~" || tag.Operator == "!~" {
			textValue = tag.Value
		} else if err == nil {
			textValue = fmt.Sprintf("%v", numericValue)
		} else {
			textValue = fmt.Sprintf("'%s'", tag.Value)
		}

		res = append(res, fmt.Sprintf(`%s"%s" %s %s`, str, tag.Key, tag.Operator, textValue))
	}

	return res
}

func (query *Query) renderTimeFilter(queryContext *tsdb.QueryContext) string {
	from := "now() - " + queryContext.TimeRange.From
	to := ""

	if queryContext.TimeRange.To != "now" && queryContext.TimeRange.To != "" {
		to = " and time < now() - " + strings.Replace(queryContext.TimeRange.To, "now-", "", 1)
	}

	return fmt.Sprintf("time > %s%s", from, to)
}

func (query *Query) renderSelectors(queryContext *tsdb.QueryContext) string {
	res := "SELECT "

	var selectors []string
	for _, sel := range query.Selects {

		stk := ""
		for _, s := range *sel {
			stk = s.Render(query, queryContext, stk)
		}
		selectors = append(selectors, stk)
	}

	return res + strings.Join(selectors, ", ")
}

func (query *Query) renderMeasurement() string {
	policy := ""
	if query.Policy == "" || query.Policy == "default" {
		policy = ""
	} else {
		policy = `"` + query.Policy + `".`
	}
	return fmt.Sprintf(` FROM %s"%s"`, policy, query.Measurement)
}

func (query *Query) renderWhereClause() string {
	res := " WHERE "
	conditions := query.renderTags()
	res += strings.Join(conditions, " ")
	if len(conditions) > 0 {
		res += " AND "
	}

	return res
}

func (query *Query) renderGroupBy(queryContext *tsdb.QueryContext) string {
	groupBy := ""
	for i, group := range query.GroupBy {
		if i == 0 {
			groupBy += " GROUP BY"
		}

		if i > 0 && group.Type != "fill" {
			groupBy += ", " //fill is so very special. fill is a creep, fill is a weirdo
		} else {
			groupBy += " "
		}

		groupBy += group.Render(query, queryContext, "")
	}

	return groupBy
}
