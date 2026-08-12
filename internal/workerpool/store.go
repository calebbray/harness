package workerpool

import (
	"database/sql"
	"fmt"
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
