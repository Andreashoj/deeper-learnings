package db_replication

import (
	"fmt"
	"math/rand"
	"time"
)

// Faking a pool of db's here, with a main / replica setup
type Pool struct {
	Main     SqlDB
	Replicas []SqlDB
}

type SqlDB struct {
	Values []User
}

type User struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func StartDatabasePool() *Pool {
	main := SqlDB{
		Values: []User{
			{Id: 1, Name: "John"},
			{Id: 2, Name: "Other John"},
		},
	}

	replicas := []SqlDB{
		{
			Values: []User{
				{Id: 1, Name: "John"},
				{Id: 2, Name: "Other John"},
			},
		},
		{
			Values: []User{
				{Id: 1, Name: "John"},
				{Id: 2, Name: "Other John"},
			},
		},
	}

	return &Pool{
		Main:     main,
		Replicas: replicas,
	}
}

func (p *Pool) Write() *SqlDB {
	return &p.Main
}

func (p *Pool) Read() *SqlDB {
	// Randomize the selection of replicas to distribute db load
	replicaIndex := rand.Intn(len(p.Replicas))
	return &p.Replicas[replicaIndex]
}

func (p *Pool) GetUser(id int) *User {
	user := User{}

	for _, u := range p.Read().Values {
		if u.Id == id {
			user = u
		}
	}

	fmt.Println("here", p.Read().Values, id)
	return &user
}

func (p *Pool) CreateUser(user User) User {
	newUser := p.Main.AddUser(user)
	p.UpdateReplicas()
	return newUser
}

func (DB *SqlDB) AddUser(user User) User {
	DB.Values = append(DB.Values, user)
	return user
}

func (p *Pool) UpdateReplicas() {
	go func() {
		time.Sleep(100 * time.Millisecond) // Emulate replication delay from main to replicas
		for i := range p.Replicas {
			p.Replicas[i].Values = make([]User, len(p.Main.Values))
			copy(p.Replicas[i].Values, p.Main.Values)
		}
	}()
}
