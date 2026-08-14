package workerpool

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/calebbray/personal-agent/internal/tools/jira"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InsertJob(instruction string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO jobs (instruction_text, status) VALUES (?, ?)`,
		instruction, Pending,
	)

	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func (s *Store) MarkRunning(id int64) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, started_at = unixepoch() WHERE id = ?`,
		Running, id,
	)
	return err
}

func (s *Store) MarkDone(id int64, result string) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, result_text = ?, finished_at = unixepoch() WHERE id = ?`,
		Done, result, id,
	)
	return err
}

func (s *Store) MarkFailed(id int64, reason string) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, result_text = ?, finished_at = unixepoch() WHERE id = ?`,
		Failed, reason, id,
	)
	return err
}

func (s *Store) GetJob(id int64) (*Job, error) {
	var j Job
	err := s.db.QueryRow(
		`SELECT id, instruction_text, status, result_text FROM jobs WHERE id = ?`,
		id,
	).Scan(&j.Id, &j.Instruction, &j.Status, &j.Result)

	if err != nil {
		return nil, err
	}

	return &j, nil

}

func (s *Store) GetJobs() ([]*Job, error) {
	out := make([]*Job, 0)
	rows, err := s.db.Query(`SELECT id, instruction_text, status, COALESCE(result_text, '') FROM jobs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.Id, &j.Instruction, &j.Status, &j.Result); err != nil {
			fmt.Printf("%v\n", err)
			continue
		}
		out = append(out, &j)
	}

	return out, nil
}

func (s *Store) MarkInterrupted() error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, result_text = ?, finished_at = unixepoch() WHERE status < ?`,
		Failed, "interrupted by restart", Done,
	)
	return err
}

func (s *Store) GetTeamIdByName(teamName string) (int, error) {
	var id int
	err := s.db.QueryRow(
		`SELECT id FROM teams WHERE label = ?`,
		teamName,
	).Scan(&id)
	if err != nil {
		return -1, err
	}
	return id, nil
}

func (s *Store) getExistingIssueCount(keys []any) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	existing := 0
	placeholders := strings.Repeat("?,", len(keys))
	placeholders = strings.TrimSuffix(placeholders, ",")

	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT issue_key FROM issues WHERE issue_key IN (%s)`, placeholders),
		keys...,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return 0, err
		}
		existing++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return existing, nil
}

func (s *Store) GetIssuesFinishedInTimeframe(teamId int, start, end int64) ([]jira.Issue, error) {
	rows, err := s.db.Query(
		`SELECT issue_key, title, loe, started_at, finished_at 
		FROM issues 
		WHERE team_id = ? AND finished_at BETWEEN ? AND ?`,
		teamId, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []jira.Issue
	for rows.Next() {
		var i jira.Issue
		if err := rows.Scan(&i.Key, &i.Title, &i.Loe, &i.StartedAt, &i.FinishedAt); err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, nil
}

func (s *Store) SaveIssues(issues []jira.Issue) (saved, updated int, err error) {
	if len(issues) == 0 {
		return 0, 0, nil
	}

	keys := make([]any, 0, len(issues))
	for _, iss := range issues {
		keys = append(keys, iss.Key)
	}

	existing, err := s.getExistingIssueCount(keys)
	if err != nil {
		return 0, 0, err
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO issues (issue_key, title, loe, team_id, started_at, finished_at, updated_at) VALUES `)
	args := make([]any, 0, len(issues)*6)
	for i, iss := range issues {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?, ?, ?, ?, ?, ?, unixepoch())")
		args = append(args, iss.Key, iss.Title, iss.Loe, iss.TeamId, iss.StartedAt, iss.FinishedAt)
	}
	sb.WriteString(`
		ON CONFLICT(issue_key) DO UPDATE SET
			title       = excluded.title,
			loe         = excluded.loe,
			team_id     = excluded.team_id,
			started_at  = COALESCE(issues.started_at, excluded.started_at),
			finished_at = COALESCE(issues.finished_at, excluded.finished_at),
			updated_at  = unixepoch()
	`)

	if _, err := s.db.Exec(sb.String(), args...); err != nil {
		return 0, 0, err
	}

	return len(issues) - existing, existing, nil
}

func (s *Store) SaveIssue(key, title string, loe int, teamId int, started_at, finished_at int64) error {
	_, err := s.db.Exec(`
		INSERT INTO issues (issue_key, title, loe, team_id, started_at, finished_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, unixepoch())
		ON CONFLICT(issue_key) DO UPDATE SET
			title       = excluded.title,
			loe         = excluded.loe,
			team_id     = excluded.team_id,
			started_at  = COALESCE(issues.started_at, excluded.started_at),
			finished_at = COALESCE(issues.finished_at, excluded.finished_at),
			updated_at  = unixepoch()
	`, key, title, loe, teamId, started_at, finished_at)
	return err
}
