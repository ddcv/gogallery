package main

import (
	"os"
	"syscall"
	"sort"
	"path"
	"fmt"
	"flag"
	"http"
	"template"
	"log"
	"regexp"
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

const maxDirDepth = 24
const thumbsDir = ".thumbs"
const picpattern = "/pic/"
const tagpattern = "/tag/"
const tagspattern = "/tags"

var (
	rootdir, _ = os.Getwd();
	rootdirlen = len(rootdir)
	// command flags 
	dbfile   = flag.String("dbfile", "./gallery.db", "File to store the db")
	host = flag.String("host", "localhost:8080", "hostname and port for this server")
	initdb    = flag.Bool("init", false, "clean out the db file and start from scratch")
	picsdir = flag.String("picsdir", "pics/", "Root dir for all the pics")
	thumbsize   = flag.String("thumbsize", "200x300", "size of the thumbnails")
	tmpldir = flag.String("tmpldir", "tmpl/", "dir for the templates")
	tagmode = flag.String("tagmode", "", "tag to use in standalone mode")
)

func scanDir(dirpath string, tag string) os.Error {
	currentDir, err := os.Open(dirpath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		return err
	}
	currentDir.Close()		
	sort.SortStrings(names)
	// used syscall because can't figure out how to check EEXIST with os
	e := 0
	e = syscall.Mkdir(path.Join(dirpath, thumbsDir), 0755) 
	if e != 0 && e != syscall.EEXIST {
		return os.Errno(e)
	}
	for _,v := range names {
		childpath := path.Join(dirpath, v)
		fi, err := os.Lstat(childpath)
		if err != nil {
			return err
		}
		if fi.IsDirectory() && v != thumbsDir {
			err = scanDir(childpath, tag)
			if err != nil {
				return err
			}
		} else {
			if picValidator.MatchString(childpath) {
				err = mkThumb(childpath)
				if err != nil {
					return err
				}
				path := childpath[rootdirlen+1:]
				insert(maxId+1, path, tag)
			}
		}

	}
	return err
}

func mkThumb(filepath string) os.Error {
	dir, file := path.Split(filepath)
	thumb := path.Join(dir, thumbsDir, file)
	fd, err := os.Open(thumb, os.O_CREAT|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		if err != os.EEXIST {
			return err
		}
		return nil
	}
	fd.Close()
	var args []string = make([]string, 5)
	args[0] = "/usr/bin/convert"
	args[1] = filepath
	args[2] = "-thumbnail"
	args[3] = *thumbsize
	args[4] = thumb
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	pid, err := os.ForkExec(args[0], args, os.Environ(), "", fds)
	if err != nil {
		return err
	}
	_, err = os.Wait(pid, os.WNOHANG)
	if err != nil {
		return err
	}
	return nil
}

func miscInit() {
	// fullpath for picsdir. must be within document root
	*picsdir = path.Clean(*picsdir)
	if (*picsdir)[0] != '/' {
		cwd, _ := os.Getwd() 
		*picsdir = path.Join(cwd, *picsdir)
	}
	pathValidator := regexp.MustCompile(rootdir + ".*")
	if !pathValidator.MatchString(*picsdir) {
		log.Exit("picsdir has to be a subdir of rootdir. (symlink ok)")
	}

	// same drill for templates.
	*tmpldir = path.Clean(*tmpldir)
	if (*tmpldir)[0] != '/' {
		cwd, _ := os.Getwd() 
		*tmpldir = path.Join(cwd, *tmpldir)
	}
	if !pathValidator.MatchString(*tmpldir) {
		log.Exit("tmpldir has to be a subdir of rootdir. (symlink ok)")
	}
	for _, tmpl := range []string{"tag", "pic", "tags"} {
		templates[tmpl] = template.MustParseFile(path.Join(*tmpldir, tmpl+".html"), nil)
	}

	if *initdb {
		initDb()
	} else {
		var err os.Error
		db, err = sqlite.Open(*dbfile)
		errchk(err)
		setMaxId()
	}
}

func errchk(err os.Error) {
	if err != nil {
		log.Exit(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr,
		"usage: gogallery -tag=sometag [-picsdir=dir] \n");
	fmt.Fprintf(os.Stderr,
		" \t gogallery \n");
	flag.PrintDefaults();
	os.Exit(2);
}

func main() {
	flag.Usage = usage
	flag.Parse()
	
	miscInit()

	// tagging mode
	if len(*tagmode) != 0 {
		errchk(scanDir(*picsdir, *tagmode))
		db.Close()
		return
	}

	// web server mode
	http.HandleFunc(tagpattern, makeHandler(tagHandler))
	http.HandleFunc(picpattern, makeHandler(picHandler))
	http.HandleFunc(tagspattern, makeHandler(tagsHandler))
	http.HandleFunc("/random", makeHandler(randomHandler))
	http.HandleFunc("/next", makeHandler(nextHandler))
	http.HandleFunc("/prev", makeHandler(prevHandler))
	http.HandleFunc("/", http.HandlerFunc(serveFile))
	http.ListenAndServe(":8080", nil)
}
