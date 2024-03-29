package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
)

//TODO: sql constraints on the id
//TODO: sanitize against injections?

var (
	db    *sql.DB
	maxId = 0
)

func initDb() {
	var err error
	db, err = sql.Open("sqlite3", config.Dbfile)
	errchk(err)
	db.Exec("drop table tags")
	_, err = db.Exec(
		"create table tags (id integer primary key, file text, tag text)")
	errchk(err)
	errchk(scanDir(config.Picsdir, allPics))
	log.Print("Scanning of " + config.Picsdir + " complete.")
}

//TODO: if insert stmt returns the id, use that to set maxId
func insert(filepath string, tag string) {
	_, err := db.Exec(
		"insert into tags values (NULL, '" +
			filepath + "', '" + tag + "')")
	if err != nil {
		// check if error was bad char in string 
		ok, newpath := badchar(filepath)
		if !ok {
			log.Fatal(err)
		}
		// retry with fixed string and rename file if ok
		_, err = db.Exec(
			"insert into tags values (NULL, '" +
				newpath + "', '" + tag + "')")
		errchk(err)
		errchk(os.Rename(path.Join(rootdir, filepath), path.Join(rootdir, newpath)))
	}
	maxId++
}

//TODO: notify the server to reset maxId
func delete(tag string) {
	_, err := db.Exec(
		`delete from tags where tag="` + tag + `"`)
	errchk(err)
}

//TODO: better sql with less redundancy?
func getNext(pic string, tag string) string {
	// use >= and limit to dodge fragmentation issues
	stmt, err := db.Prepare(
		`select file, tag from tags where id > ` +
			`(select id from tags where file = '` + pic +
			`' and tag = '` + tag + `')` +
			" and tag = '" + tag + "' order by id asc limit 1")
	errchk(err)

	s := ""
	s2 := ""
	res, err := stmt.Query()
	errchk(err)
	if res.Next() {
		errchk(res.Scan(&s, &s2))
	}
	if s2 != tag {
		// we reached the end of this tag's group
		return pic
	}
	res.Close()
	return s
}

//TODO: better sql with less redundancy?
func getPrev(pic string, tag string) string {
	// use <= and limit to dodge fragmentation issues
	stmt, err := db.Prepare(
		"select file, tag from tags where id < " +
			"(select id from tags where file = '" + pic +
			"' and tag = '" + tag + "')" +
			" and tag = '" + tag + "' order by id desc limit 1")
	errchk(err)

	s := ""
	s2 := ""
	res, err := stmt.Query()
	errchk(err)
	if res.Next() {
		errchk(res.Scan(&s, &s2))
	}
	if s2 != tag {
		// we reached the beginning of this tag's group
		return pic
	}
	res.Close()
	return s
}

func getNextId(id int) string {
	// use >= and limit to dodge fragmentation issues
	stmt, err := db.Prepare(
		"select file from tags where id > " + fmt.Sprint(id) +
			" order by id asc limit 1")
	errchk(err)

	s := ""
	res, err := stmt.Query()
	errchk(err)
	if res.Next() {
		errchk(res.Scan(&s))
	}
	res.Close()
	return s
}

func getPrevId(id int) string {
	// use <= and limit to dodge fragmentation issues
	stmt, err := db.Prepare(
		"select file from tags where id < " + fmt.Sprint(id) +
			" order by id desc limit 1")
	errchk(err)

	s := ""
	res, err := stmt.Query()
	errchk(err)
	if res.Next() {
		errchk(res.Scan(&s))
	}
	res.Close()
	return s
}

func getCurrentId(filepath string) int {
	stmt, err := db.Prepare(
		"select id from tags where file = '" + filepath + "'")
	errchk(err)
	res, err := stmt.Query()
	errchk(err)
	var i int
	if res.Next() {
		errchk(res.Scan(&i))
	}
	res.Close()
	return i
}

func setMaxId() {
	// check db sanity
	stmt, err := db.Prepare("select count(id) from tags")
	errchk(err)
	res, err := stmt.Query()
	errchk(err)
	var i int
	if res.Next() {
		errchk(res.Scan(&i))
	}
	res.Close()
	if i == 0 {
		log.Fatal("empty db. fill it with with -init or -tagmode")
	}
	// now do the real work
	stmt, err = db.Prepare("select max(id) from tags")
	errchk(err)
	res, err = stmt.Query()
	errchk(err)
	//BUG: Next() returns true when select max(id)... results in an empty set
	if res.Next() {
		errchk(res.Scan(&maxId))
	}
	res.Close()
}

//TODO: use the count to set the tags sizes
func getTags() lines {
	stmt, err := db.Prepare(
		"select tag, count(tag) from tags group by tag")
	errchk(err)

	var s string
	var i int
	var tags lines
	res, err := stmt.Query()
	errchk(err)
	for res.Next() {
		errchk(res.Scan(&s, &i))
		tags.Write(s)
	}
	res.Close()
	return tags
}

func getPics(tag string) lines {
	stmt, err := db.Prepare(
		"select file from tags where tag = '" + tag + "'")
	errchk(err)

	var s string
	var pics lines
	res, err := stmt.Query()
	errchk(err)
	for res.Next() {
		errchk(res.Scan(&s))
		pics.Write(s)
	}
	res.Close()
	return pics
}
