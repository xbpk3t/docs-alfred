package gh

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v56/github"

	_ "github.com/mattn/go-sqlite3"
)

var respositorySql = `
CREATE TABLE IF NOT EXISTS repository (
    id varchar(255) PRIMARY KEY,
    url varchar(255) NOT NULL,
    user varchar(255) NOT NULL,
    name varchar(255) NOT NULL,
    homepage varchar(255) NOT NULL,
    description text,
    pushed_at timestamp,
    created_at timestamp,
    updated_at timestamp
)`

func OpenDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err = db.Exec(respositorySql); err != nil {
		return nil, err
	}

	return db, nil
}

func (r Repository) FullName() string {
	return fmt.Sprintf("%s/%s", r.User, r.Name)
}

// Search from sqlite
// func (rs Repos) ListRepositories(localDB string) (Repos, error) {
// 	db, err := OpenDB(localDB)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	rows, err := db.Query("SELECT id, url,description, name,user,updated_at FROM repository")
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	for rows.Next() {
// 		var id, url, descr, name, user string
// 		var updated time.Time
// 		err = rows.Scan(&id, &url, &descr, &name, &user, &updated)
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		rs = append(rs, Repository{
// 			URL:         url,
// 			Type:        name,
// 			User:        user,
// 			Description: descr,
// 			LastUpdated: updated,
// 			IsStar:      false,
// 		})
// 	}
//
// 	return rs, nil
// }

func NewRepos() Repos {
	return Repos{}
}

func (rs *Repos) ListRepositories(localDB string) error {
	db, err := OpenDB(localDB)
	if err != nil {
		slog.Error("Failed to Open DB", slog.Any("Error", err))
		return err
	}

	rows, err := db.Query("SELECT id, url, description, name, user, homepage, updated_at FROM repository")
	if err != nil {
		slog.Error("Failed to Query", slog.Any("Error", err))
		return err
	}

	for rows.Next() {
		var id, url, descr, name, user, homepage string
		var updated time.Time
		err = rows.Scan(&id, &url, &descr, &name, &user, &homepage, &updated)
		if err != nil {
			return err
		}
		*rs = append(*rs, Repository{
			URL:         url,
			Name:        name,
			User:        user,
			Des:         descr,
			Doc:         homepage,
			LastUpdated: updated,
			IsStar:      false,
		})
	}

	return nil
}

func (rs Repos) UpdateRepositories(token, localDB string) (int64, error) {
	// my rs
	userRepos, err := NewGithubClient(token).ListUserRepositories()
	if err != nil {
		slog.Error("Failed to list my github repositories", slog.Any("Error", err))
		return 0, err
	}

	// starred rs
	// starredRepos, err := NewGithubClient(token).ListStarredRepositories()
	// if err != nil {
	// 	slog.Error("Failed to list my starred github repositories", slog.Any("Error", err))
	// 	return 0, err
	// }

	db, err := OpenDB(localDB)
	if err != nil {
		slog.Error("Failed to open database", slog.Any("Error", err))
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	found := map[string]struct{}{}
	counter := int64(0)

	for _, repo := range userRepos {
		slog.Info("Updating", slog.Any("Repo", fmt.Sprintf("%s/%s", *repo.Owner.Login, *repo.Name)))

		name := fmt.Sprintf("%s/%s", *repo.Owner.Login, *repo.Name)
		res, err := db.Exec(
			`INSERT OR REPLACE INTO repository (
					id,
					url,
					description,
					name, user,
				    homepage,
					pushed_at,
					updated_at,
					created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name,
			nilableString(repo.HTMLURL),
			nilableString(repo.Description),
			*repo.Name,
			*repo.Owner.Login,
			nilableString(repo.Homepage),
			githubTime(repo.PushedAt),
			githubTime(repo.UpdatedAt),
			githubTime(repo.CreatedAt),
		)
		if err != nil {
			slog.Error("Failed to Insert repository", slog.Any("Error", err))
			return counter, err
		}
		found[name] = struct{}{}
		rows, _ := res.RowsAffected()
		counter += rows
	}

	err = rs.ListRepositories(localDB)
	if err != nil {
		slog.Error("Failed to ListRepositories()", slog.Any("Error", err))
		return 0, err
	}

	// purge rs that don't exit any more
	for _, repo := range rs {
		if _, exists := found[repo.FullName()]; !exists {
			slog.Info("Repo not exist, deleting", slog.Any("Repo", repo.FullName()))

			_, err := db.Exec(
				`DELETE FROM repository WHERE id=?`,
				repo.FullName(),
			)
			if err != nil {
				return 0, err
			}

		}
	}

	return counter, tx.Commit()
}

func nilableString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func githubTime(t *github.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	return &t.Time
}
