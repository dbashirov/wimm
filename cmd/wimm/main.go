package main

import (
	"database/sql"
	"fmt"
	model "wimm/internal/model/wimm"
	"wimm/pkg/test"

	_ "github.com/lib/pq"
)

const (
	// Initialize connection constants.
	HOST     = "localhost"
	PORT     = 5432
	DATABASE = "golanged"
	USER     = "damirbasirov"
	PASSWORD = ""
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

type gUser struct {
	id   int
	name string
	age  uint16
}

func addUser(db *sql.DB, name string, id, age int) {
	sql_statement := "INSERT INTO users (id, name, age) VALUES ($1, $2, $3);"
	_, err := db.Exec(sql_statement, id, name, age)
	checkError(err)
}

func getUsers(db *sql.DB, users *[]gUser) {
	rows, err := db.Query("SELECT * FROM USERS")
	checkError(err)

	for rows.Next() {
		u := gUser{}
		err := rows.Scan(&u.id, &u.name, &u.age)
		if err != nil {
			fmt.Println(err)
			continue
		}
		*users = append(*users, u)
	}
}

func main() {

	test.Echo("Hello")

	// Initialize connection string.
	var connectionString string = fmt.Sprintf("host=%s port=%d dbname=%s user=%s sslmode=disable", HOST, PORT, DATABASE, USER)

	// Initialize connection object.
	db, err := sql.Open("postgres", connectionString)
	checkError(err)

	defer db.Close()

	// connection status check
	err = db.Ping()
	checkError(err)
	fmt.Println("Successfully created connection to database")

	// get all users
	users := []gUser{}
	getUsers(db, &users)
	for _, u := range users {
		fmt.Println(u.id, u.name, u.age)
	}

	// users = []gUser{}
	// addUser(db, "Stepa 2", 6, 30)
	// getUsers(db, &users)
	// for _, u := range users {
	// 	fmt.Println(u.id, u.name, u.age)
	// }

	typeWallet := model.TypeExpense
	fmt.Println(typeWallet)
}
