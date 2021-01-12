package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type AflExporer struct {
	RootFolder string
}

type QueueFile struct {
	Id string
	Source []string
	File_type string
	New_coverage bool
	Filename string
}

type SigmaNode struct {
	Id string		`json:"id"`
	Label string  	`json:"label"`
	X float32		`json:"x"`
	Y float32		`json:"y"`
	Size float32	`json:"size"`
	Color string	`json:"color"`
}

type SigmaEdge struct {
	Id	string		`json:"id"`
	Source string	`json:"source"`
	Taget string 	`json:"target"`
	Type string		`json:"type"`
	Size float32	`json:"size"`
	Color string 	`json:"color"`
	Label string 	`json:"label"`
}

type SigmaGraph struct {
	Nodes []SigmaNode	`json:"nodes"`
	Edges []SigmaEdge	`json:"edges"`
}


func getColor(qf QueueFile) (string) {
	// Queue File
	if strings.Compare(qf.File_type, "queue") == 0 {
		if qf.New_coverage {
			return "#32CD32"
		}
		return "#1E90FF"
	}

	if strings.Compare(qf.File_type, "crashes") == 0 {
		return "#B22222"
	}

	if strings.Compare(qf.File_type, "hangs") == 0 {
		return "#FF6347"
	}

	// should not be reached maybe panic?
	return "a"
}

func parse_filename(filename string, file_type string) (queue_file QueueFile) {

	split_filename := strings.Split(filename, ",")

	// check id
	// expected id:000005
	id := strings.Split(split_filename[0], ":")[1]

	// check if original file from input folder
	if strings.Contains(split_filename[len(split_filename)-1], "orig") ||
	strings.Contains(split_filename[1], "sync")	{
		queue_file = QueueFile{
			Id: file_type + "/id:" + id,
			Source: []string{},
			File_type: file_type,
			New_coverage: false,
			Filename: filename,
		}
		return queue_file
	}

	// check source
	// position can vary
	source := []string{}
	for _, s := range split_filename {
		if strings.Contains(s, "src:"){
			src := strings.Split(s, ":")[1]
			source = strings.Split(src, "+")
			break
		}
	}

	// if last == "+cov" we have new coverage
	new_cov := false
	if strings.Compare(split_filename[len(split_filename)-1], "+cov") == 0 {
		new_cov = true
	}

	queue_file = QueueFile{
		Id: file_type + "/id:" + id,
		Source: source,
		File_type: file_type,
		New_coverage: new_cov,
		Filename: filename,
	}

	return queue_file
}

func parse_fuzzer_instance(root_folder string) ([]QueueFile) {
	queue_files := parse_folder(path.Join(root_folder, "queue"))
	queue_files = append(queue_files, parse_folder(path.Join(root_folder, "crashes"))...)
	queue_files = append(queue_files, parse_folder(path.Join(root_folder, "hangs"))...)
	return queue_files
}

func parse_folder(foldername string) ([]QueueFile) {

	queue_files := []QueueFile{}

	f_type := strings.Split(foldername, "/")
	file_type := f_type[len(f_type)-1]

	files, err := ioutil.ReadDir(foldername)
	if err != nil {
		log.Fatalf("ReadDir failed: %s", err)
		//log.Fatal(err)
	}

	for _, filename := range files {
		// check if not .state file or README.txt...
		if !strings.Contains(filename.Name(), "id:") {
			continue
		}
		qf := parse_filename(filename.Name(), file_type)
		queue_files = append(queue_files, qf)
	}
	return queue_files
}

func export_to_sigma_json(queue_files []QueueFile) (string) {
	nodes := []SigmaNode{}
	edges := []SigmaEdge{}
	edge_id := 0

	for i, qf := range queue_files {
		// Create Node
		node := SigmaNode{
			//Id: qf.File_type + "/" + qf.Id,
			Id: qf.Id,
			//Id: qf.File_type + "/" +qf.Filename,
			Label: qf.File_type + "/" +qf.Filename,
			X: rand.Float32() * float32(len(queue_files)),
			Y: rand.Float32() * (float32(i)),
			Size: 1.0,
			Color: getColor(qf),
		}

		nodes = append(nodes, node)

		// For loop create edges
		for _, source := range qf.Source {
			edge := SigmaEdge{
				Id: strconv.Itoa(edge_id),
				//Source: "queue/" + source,
				Source: "queue/id:" + source,
				Taget: qf.Id,
				Label: "queue/" + source + "|",
				Type: "arrow",
				Size: 2.0,
				Color: getColor(qf),
			}
			edges = append(edges, edge)
			edge_id++
		}
	}

	graph := SigmaGraph{Nodes: nodes, Edges: edges}

	graph_json, _ := json.Marshal(graph)
	return string(graph_json)
}

func (AflExp *AflExporer) data(w http.ResponseWriter, req *http.Request) {
	fmt.Println(AflExp.RootFolder)
	all := parse_fuzzer_instance(AflExp.RootFolder)
	json := export_to_sigma_json(all)
	fmt.Fprintln(w, json)
}

func index(w http.ResponseWriter, req *http.Request) {
	//file, _ := exec.LookPath(os.Args[0])
	//fmt.Print(file)
	//data, err := ioutil.ReadFile(path.Join(file, "templates/index.html"))
	data, err := ioutil.ReadFile("templates/index.html")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(w, string(data))
}

// receives the label of an edge. However this information if not enough to construct the full path
// Glob is used to find the right files
func (AflExp *AflExporer) diff(w http.ResponseWriter, req *http.Request) {
	keys:= req.URL.Query()
	f1 := keys.Get("f1")
	f2 := keys.Get("f2")
	if f1 == "" || f2 == "" {
		fmt.Fprintln(w, "An argument is missing!")
		return
	}

	if strings.Contains(f1, "..") || strings.Contains(f2, "..") {
		fmt.Fprint(w, "Only paths from root dir allowed")
		return
	}

	f1_files, err1 := filepath.Glob(path.Join(AflExp.RootFolder, f1) + "*")
	f2_files, err2 := filepath.Glob(path.Join(AflExp.RootFolder, f2) + "*")
	if err1 != nil || err2 != nil {
		fmt.Println(err1)
		fmt.Println(err2)
		fmt.Fprint(w, "Error finding files")
		return
	}
	if len(f1_files) == 0 || len(f2_files) == 0 {
		fmt.Fprint(w, "Error finding files")
		return
	}

	f1_file := f1_files[0]
	f2_file := f2_files[0]


	f1_buf, err1 := ioutil.ReadFile(path.Join(f1_file))
	f2_buf, err2 := ioutil.ReadFile(path.Join(f2_file))

	if err1 != nil || err2 != nil {
		fmt.Fprintln(w, "A file could not be read.")
		fmt.Println(err1)
		fmt.Println(err2)
		return
	}

	e1 := make([]byte, hex.EncodedLen(len(f1_buf)))
	hex.Encode(e1, f1_buf)

	e2 := make([]byte, hex.EncodedLen(len(f2_buf)))
	hex.Encode(e2, f2_buf)

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(e1), string(e2), false)

	htmlDiff := dmp.DiffPrettyHtml(diffs)
	w.Header().Set("Content-Type", "text/html")

	fmt.Fprintf(w, "<span style='display:block; width:500px; word-wrap:break-word; font-family: monospace'>%s</span>", htmlDiff)
}

// Receives the label of a node and return the hexdump of the file
// Node Labels contain the full path already!
func (AflExp *AflExporer) show(w http.ResponseWriter, req *http.Request) {
	keys:= req.URL.Query()
	f := keys.Get("f")
	if f == "" {
		fmt.Fprintln(w, "f parameter is missing")
		return
	}

	if strings.Contains(f, "..") {
		fmt.Fprint(w, "Only paths from root dir allowed")
		return
	}

	f_buf, err := ioutil.ReadFile(path.Join(AflExp.RootFolder, f))
	if err != nil {
		fmt.Println(err)
		fmt.Fprintln(w, "Error reading file")
	}

	w.Header().Set("Content-Type", "text/html")

	fmt.Fprintf(w, "<pre>%s </pre>", html.EscapeString(hex.Dump(f_buf)))
}

func main() {

	rootFolderPtr := flag.String("in", "", "Path to root folder")
	listenAddrPtr := flag.String("listen", "localhost:8080", "Address to listen on, default: 'localhost:8080'")

	flag.Parse()

	aflExporer := &AflExporer{RootFolder: *rootFolderPtr}

	http.HandleFunc("/data.json", aflExporer.data)
	http.HandleFunc("/", index)
	http.HandleFunc("/diff", aflExporer.diff)
	http.HandleFunc("/show", aflExporer.show)

	//http.ListenAndServe("localhost:8080", nil)
	http.ListenAndServe(*listenAddrPtr, nil)
}
