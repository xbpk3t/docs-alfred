package gh

import (
	"database/sql"
	"fmt"
	"log"
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
// 			Name:        name,
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
		return err
	}

	rows, err := db.Query("SELECT id, url, description, name, user, updated_at FROM repository")
	if err != nil {
		return err
	}

	for rows.Next() {
		var id, url, descr, name, user string
		var updated time.Time
		err = rows.Scan(&id, &url, &descr, &name, &user, &updated)
		if err != nil {
			return err
		}
		*rs = append(*rs, Repository{
			URL:         url,
			Name:        name,
			User:        user,
			Des:         descr,
			LastUpdated: updated,
			IsStar:      false,
		})
	}

	return nil
}

// func (rs Repos) RemoveDuplicates() Repos {
// 	uniqueValues := make(map[string]bool)
// 	// result := make([]Repository, 0)
//
// 	for _, t := range rs {
// 		if !uniqueValues[t.URL] {
// 			uniqueValues[t.URL] = true
// 			rs = append(rs, t)
// 		}
// 	}
//
// 	return rs
// }

// func addMarkdownListFormat(str []string) string {
// 	var builder strings.Builder
// 	for _, str := range str {
// 		builder.WriteString(fmt.Sprintf("- %s\n", str))
// 	}
// 	return builder.String()
// }

func (rs Repos) UpdateRepositories(token, localDB string) (int64, error) {
	// my rs
	userRepos, err := NewGithubClient(token).ListUserRepositories()
	if err != nil {
		return 0, err
	}

	// starred rs
	starredRepos, err := NewGithubClient(token).ListStarredRepositories()
	if err != nil {
		return 0, err
	}

	db, err := OpenDB(localDB)
	if err != nil {
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	found := map[string]struct{}{}
	counter := int64(0)

	for _, repo := range append(userRepos, starredRepos...) {
		log.Printf("Updating %s/%s", *repo.Owner.Login, *repo.Name)

		name := fmt.Sprintf("%s/%s", *repo.Owner.Login, *repo.Name)
		res, err := db.Exec(
			`INSERT OR REPLACE INTO repository (
					id,
					url,
					description,
					name, user,
					pushed_at,
					updated_at,
					created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			name,
			nilableString(repo.HTMLURL),
			nilableString(repo.Description),
			*repo.Name,
			*repo.Owner.Login,
			githubTime(repo.PushedAt),
			githubTime(repo.UpdatedAt),
			githubTime(repo.CreatedAt),
		)
		if err != nil {
			return counter, err
		}
		found[name] = struct{}{}
		rows, _ := res.RowsAffected()
		counter += rows
	}

	err = rs.ListRepositories(localDB)
	if err != nil {
		return 0, err
	}

	// purge rs that don't exit any more
	for _, repo := range rs {
		if _, exists := found[repo.FullName()]; !exists {
			log.Printf("Repo %s doesn't exist, deleting", repo.FullName())

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
