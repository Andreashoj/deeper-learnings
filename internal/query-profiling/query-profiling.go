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
	Id       int    `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type Permission struct {
	Id   int
	Role string
}

type Subscription struct {
	Id        int
	StartDate time.Time
	EndDate   time.Time
	Status    string
	UserID    int
}

func InsertUsersAndPosts() {
	query := `
		DROP TABLE IF EXISTS subscriptions;
		DROP TABLE IF EXISTS users_permissions;
		DROP TABLE IF EXISTS posts;
		DROP TABLE IF EXISTS users;
		DROP TABLE IF EXISTS permissions;

		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			username VARCHAR(255) NOT NULL,
			password VARCHAR(255) NOT NULL
		);

		CREATE TABLE permissions (
		    id SERIAL PRIMARY KEY,
		    role VARCHAR(255)
		);

		CREATE TABLE users_permissions (
		    user_id INT NOT NULL,
		    permission_id INT NOT NULL,
			CONSTRAINT fk_user
				FOREIGN KEY (user_id)
					REFERENCES users(id),
			CONSTRAINT fk_permission
				FOREIGN KEY (permission_id)
					REFERENCES permissions(id)
		);

		CREATE TABLE subscriptions (
		    id SERIAL PRIMARY KEY,
			start_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			end_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		    status VARCHAR(255) NOT NULL,
			user_id INT NOT NULL,
			CONSTRAINT fk_user
				FOREIGN KEY (user_id)
					REFERENCES users(id)
		);

		CREATE TABLE posts (
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
		userID := i
		permissionID := i
		_, err = db.DB.Exec("INSERT INTO users (name, username, password) VALUES ($1, $2, $3);", userID, "anz", "tester123")
		if err != nil {
			fmt.Printf("failed seeding users: %s", err)
			return
		}
		_, err = db.DB.Exec("INSERT INTO posts (name, user_id) VALUES ($1, $2);", i, userID)
		if err != nil {
			fmt.Printf("failed seeding posts: %s", err)
			return
		}

		_, err = db.DB.Exec("INSERT INTO permissions (role) VALUES ($1);", fmt.Sprintf("role-type-%v", i))
		if err != nil {
			fmt.Printf("failed seeding permissions: %s", err)
			return
		}

		_, err = db.DB.Exec("INSERT INTO users_permissions (user_id, permission_id) VALUES ($1, $2);", userID, permissionID)
		if err != nil {
			fmt.Printf("failed seeding users_permissions: %s", err)
			return
		}

		nextMonth := time.Now().AddDate(0, 1, 0)
		_, err = db.DB.Exec("INSERT INTO subscriptions (end_date, status, user_id) VALUES ($1, 'active', $2);", nextMonth, userID)
		if err != nil {
			fmt.Printf("failed seeding subscriptions: %s", err)
			return
		}
	}
}

func StartQueryProfiling() {
	InsertUsersAndPosts()

	_, _, err := GetPostsAndUsersNPlus()
	if err != nil {
		fmt.Printf("failed getting posts and users with NPLUS issue: %s", err)
		return
	}

	_, _, err = GetPostsAndUsersWithoutNPlus()
	if err != nil {
		fmt.Printf("failed getting posts and users with NPLUS issue: %s", err)
		return
	}

	explainQuery("SELECT id, name, user_id FROM posts")

	// explainQuery("SELECT name FROM users WHERE id = 1")
}

func GetPostsAndUsersWithoutNPlus() ([]Post, []User, error) {
	now := time.Now()
	posts := []Post{}
	users := []User{}

	rows, err := db.DB.Query("SELECT posts.id, posts.name, users.id, users.name FROM posts LEFT JOIN users ON posts.user_id = users.id")
	if err != nil {
		return nil, nil, fmt.Errorf("failed getting posts: %w", err)
	}

	for rows.Next() {
		post := Post{}
		user := User{}

		if err := rows.Scan(&post.Id, &post.Name, &user.Id, &user.Name); err != nil {
			return nil, nil, fmt.Errorf("failed mapping post and id: %w", err)
		}

		posts = append(posts, post)
		users = append(users, user)
	}

	fmt.Println(time.Since(now))

	return posts, users, nil
}

func GetPostsAndUsersNPlus() ([]Post, []User, error) {
	now := time.Now()
	posts, err := GetPosts()
	if err != nil {
		return nil, nil, fmt.Errorf("failed getting posts: %w", err)
	}

	users := []User{}
	for _, post := range posts {
		user, err := GetUser(post.UserID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed getting user: %w", err)
		}
		users = append(users, *user)
	}

	fmt.Println(time.Since(now))

	return posts, users, nil
}

func GetPosts() ([]Post, error) {
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

func GetUser(id int) (*User, error) {
	user := User{Id: id}
	err := db.DB.QueryRow("SELECT name FROM users WHERE id = $1", user.Id).Scan(&user.Name)
	if err != nil {
		return nil, fmt.Errorf("failed getting posts: %w", err)
	}

	return &user, nil
}

func GetUsers() ([]User, error) {
	rows, err := db.DB.Query("SELECT name FROM users")
	if err != nil {
		return nil, err
	}

	var users []User
	for rows.Next() {
		var user User
		if err = rows.Scan(&user.Id, &user.Name); err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, nil
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
