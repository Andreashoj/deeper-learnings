package query_profiling

import (
	"fmt"
	"time"

	"andreashoj/deeper-learnings/internal/db"
)

type Post struct {
	Id     int
	Name   string
	UserID int
}

type User struct {
	Id   int
	Name string
}

func StartQueryProfiling() {
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);

		CREATE TABLE IF NOT EXISTS posts (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255),
			user_id INT NOT NULL,
			CONSTRAINT fk_user
				FOREIGN KEY (user_id)
					REFERENCES users(id)
		);

	`

	_, err := db.DB.Exec(query)
	if err != nil {
		fmt.Printf("failed seeding init query: %s", err)
		return
	}

	for i := 1; i <= 1000; i++ {
		_, err = db.DB.Exec("INSERT INTO users (name) VALUES ($1);", i)
		if err != nil {
			fmt.Printf("failed seeding users: %s", err)
			return
		}
		_, err = db.DB.Exec("INSERT INTO posts (name, user_id) VALUES ($1, $2);", i, i)
		if err != nil {
			fmt.Printf("failed seeding posts: %s", err)
			return
		}
	}

	if err := getPostsAndUsersNPlus(); err != nil {
		fmt.Printf("failed getting posts and users with NPLUS issue: %s", err)
		return
	}

	if err := getPostsAndUsersWithoutNPlus(); err != nil {
		fmt.Printf("failed getting posts and users with NPLUS issue: %s", err)
		return
	}

	explainQuery("SELECT id, name, user_id FROM posts")

	// explainQuery("SELECT name FROM users WHERE id = 1")
}

func getPostsAndUsersWithoutNPlus() error {
	now := time.Now()
	posts := []Post{}
	users := []User{}

	rows, err := db.DB.Query("SELECT posts.id, posts.name, users.id, users.name FROM posts LEFT JOIN users ON posts.user_id = users.id")
	if err != nil {
		return fmt.Errorf("failed getting posts: %w", err)
	}

	for rows.Next() {
		post := Post{}
		user := User{}

		if err := rows.Scan(&post.Id, &post.Name, &user.Id, &user.Name); err != nil {
			return fmt.Errorf("failed mapping post and id: %w", err)
		}

		posts = append(posts, post)
		users = append(users, user)
	}

	fmt.Println(time.Since(now))

	return nil
}

func getPostsAndUsersNPlus() error {
	now := time.Now()
	posts, err := getPosts()
	if err != nil {
		return fmt.Errorf("failed getting posts: %w", err)
	}

	users := []User{}
	for _, post := range posts {
		user, err := getUser(post.UserID)
		if err != nil {
			return fmt.Errorf("failed getting user: %w", err)
		}
		users = append(users, *user)
	}

	fmt.Println(time.Since(now))

	return nil
}

func getPosts() ([]Post, error) {
	rows, err := db.DB.Query("SELECT id, name, user_id FROM posts")
	if err != nil {
		return nil, fmt.Errorf("failed getting posts: %w", err)
	}

	posts := []Post{}
	for rows.Next() {
		post := Post{}

		if err := rows.Scan(&post.Id, &post.Name, &post.UserID); err != nil {
			return nil, fmt.Errorf("failed mapping post row: %w", err)
		}

		posts = append(posts, post)
	}

	return posts, nil
}

func getUser(id int) (*User, error) {
	user := User{Id: id}
	err := db.DB.QueryRow("SELECT name FROM users WHERE id = $1", user.Id).Scan(&user.Name)
	if err != nil {
		return nil, fmt.Errorf("failed getting posts: %w", err)
	}

	return &user, nil
}

func explainQuery(query string) {
	rows, err := db.DB.Query("EXPLAIN ANALYZE " + query)
	if err != nil {
		fmt.Print("failed analyzing query: %s", err)
		return
	}

	for rows.Next() {
		defer rows.Close()
		var plan string
		if err = rows.Scan(&plan); err != nil {
			fmt.Printf("failed getting query plan: %s", err)
			return
		}
		fmt.Println(plan)
	}
}
