package jira

import "time"

type changelog struct {
	Changelog struct {
		Histories []struct {
			Timestamp string `json:"created"`
			Items     []struct {
				Field   string `json:"field"`
				From    string `json:"from"`
				FromStr string `json:"fromString"`
				To      string `json:"to"`
				ToStr   string `json:"toString"`
			} `json:"items"`
		} `json:"histories"`
	} `json:"changelog"`
}

func (cl changelog) finish(categories statusCategories) (int64, bool) {
	history := cl.Changelog.Histories
	for i := len(history) - 1; i >= 0; i-- {
		for _, item := range history[i].Items {
			if item.Field != "status" {
				continue
			}

			fromCat := categories[item.From]
			toCat := categories[item.To]
			if fromCat != "done" && toCat == "done" {
				return parseIsoTimestamp(history[i].Timestamp), true
			}
		}
	}
	return 0, false
}

func (cl changelog) start(categories statusCategories) (int64, bool) {
	history := cl.Changelog.Histories
	for i := len(history) - 1; i >= 0; i-- {
		for _, item := range history[i].Items {
			if item.Field != "status" {
				continue
			}

			fromCat := categories[item.From]
			toCat := categories[item.To]
			if fromCat == "new" && toCat != "new" {
				return parseIsoTimestamp(history[i].Timestamp), true
			}
		}
	}
	return 0, false
}

func parseIsoTimestamp(stamp string) int64 {
	t, _ := time.Parse("2006-01-02T15:04:05.000-0700", stamp)
	return t.UnixNano()
}

type cycleTimeResponse struct {
	Issues  map[string]string `json:"issues"`
	Metrics metrics           `json:"metrics"`
}

type jiraIssue struct {
	Key string `json:"key"`
}

type board struct {
	Values []struct {
		SprintUrl string `json:"self"`
	} `json:"values"`
}

type sprint struct {
	Issues []jiraIssue `json:"issues"`
}

type cycleReport struct {
	Key       string
	CycleTime time.Duration
}
