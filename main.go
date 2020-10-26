package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var mux map[string]func(http.ResponseWriter, *http.Request)

func main() {
	major, minor, patch := 0, 0, 4
	fmt.Printf("%s v%d.%d.%d\n", filepath.Base(os.Args[0]), major, minor, patch)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	port := flag.Int("p", 8000, "service port")
	ver := flag.Bool("v", false, "show version and exit")
	flag.Parse()
	if *ver {
		os.Exit(0)
	}
	server := http.Server{
		Addr:        fmt.Sprintf(":%d", *port),
		Handler:     &Handler{},
		ReadTimeout: 10 * time.Second,
	}
	mux = map[string]func(http.ResponseWriter, *http.Request){
		"/": uploadPage,
	}
	fmt.Printf("http server @ %s\n", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("listen&serve failed: %v", err)
	}
}

type Handler struct{}

func (*Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler, ok := mux[r.URL.String()]; ok {
		handler(w, r)
	} else {
		http.StripPrefix("/", http.FileServer(http.Dir("./"))).ServeHTTP(w, r)
	}
}

func uploadPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getHandler(w, r)
	case "POST":
		postHandler(w, r)
	default:
		fmt.Fprintf(w, "unknown HTTP method: %v", r.Method)
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(1 << 26) // 64M
	file, handler, err := r.FormFile("uploadfile")
	if err != nil {
		fmt.Fprintf(w, "upload failed: %v", err)
		return
	}
	filename := handler.Filename
	f, err := os.OpenFile(filepath.Join(".", filename), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		fmt.Fprintf(w, "upload failed: %v", err)
		return
	}
	defer f.Close()
	_, err = io.Copy(f, file)
	if err != nil {
		fmt.Fprintf(w, "upload failed: %v", err)
		return
	}
	fmt.Fprintf(w, "upload completed: %s", filename)
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html")

	reqPath := filepath.Join("./", r.URL.Path)
	fmt.Printf("getting: %s\n", reqPath)

	fd, err := os.Open(reqPath)
	if err != nil {
		fmt.Fprintf(w, "cannot open %s, error: %v", reqPath, err)
		return
	}
	fileInfo, err := fd.Stat()
	if err != nil {
		fmt.Fprintf(w, "cannot stat %s, error: %v", reqPath, err)
		return
	}
	if fileInfo.IsDir() {
		fileList := getFilelist(reqPath, fileInfo.Name())
		fileLinkList := getFileLinks(reqPath, fileList)
		w.Write(genBody(fileLinkList))
	} else {
		fileData, err := ioutil.ReadFile(reqPath)
		if err != nil {
			fmt.Fprintf(w, "cannot read %s, error: %v", reqPath, err)
			return
		}
		w.Write(fileData)
	}
}

func genBody(body string) []byte {
	return []byte(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>HTTP Server</title>
  <style>*{color:#1e1e1e;background:#efefef;}</style>
</head>
<body>
` + body + `
</body>
</html>
`)
}

func getFilelist(baseDir, filename string) (list []string) {
	fd, err := os.Open(filepath.Join(baseDir, filename))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer fd.Close()
	infos, err := fd.Readdir(0)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, info := range infos {
		list = append(list, info.Name())
	}
	sort.Strings(list)
	return
}

func getFileLinks(baseDir string, files []string) (list string) {
	list = fmt.Sprintf(`
  <h3>Directory listing for %s</h3>
  <form method="post" enctype="multipart/form-data" action="/">
	<label for="upload">upload file:</label>
	<input type="file" id="upload" name="uploadfile" />
	<input type="submit" value="Submit" />
  </form><hr><ul>`, baseDir)
	for _, file := range files {
		list += fmt.Sprintf(`<li><a href="%s">%s</a></li>`, filepath.Join(baseDir, file), file)
	}
	list += "</ul><hr>"
	return
}
