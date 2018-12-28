package main

import (
	"sync/atomic"
	"path"
	"path/filepath"
	"time"
	"flag"
	"log"
	"fmt"
	"io"
	"os"
	"os/exec"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http service address")
var docroot = flag.String("path", ".", "http root directory")
var urlprefix = flag.String("prefix", "/rpm", "URL repository directory")
var iphead = flag.String("ip", "", "header for remote IP")
var metagen = flag.String("metagen", "createrepo", "tool to call for metadata update")

type opts struct {
	UrlPrefix string
	Path string
	IpHeader string
}

var notifier = make(chan string, 64);

func t64() uint64 {
	return uint64(time.Now().UnixNano());
}

func tdelta(ts uint64) string {
	td := t64()-ts;
	return fmt.Sprintf("%d.%03ds",
		td / 1000000000,
		(td / 1000000) % 1000);
}

func timestamp() string {
	t := time.Now()
	yy, mn, dy := t.Date()
	hh, mm, ss := t.Clock()
	return fmt.Sprintf(
		"%04d%02d%02d-%02d:%02d:%02d",
		yy, mn, dy, hh, mm, ss);
}

func main() {
	go func () {
		for {

			dirs := make(map[string]bool);
			flag := false;
			for {
				select {
				case s := <- notifier:
					// fmt.Printf("-- got: '%v'\n", s);
					dirs[s]=true;
					flag = true;
				default:
					// fmt.Printf("sleep\n");
					if (!flag) {
						for k, _ := range dirs {
							// fmt.Printf("-- notify: '%v'\n", k);
							fmt.Printf("-- metadata run in: %v\n", k);
							ts := t64();
							cmd := exec.Command(*metagen, k);
							cmd.Stdout = os.Stdout;
							cmd.Stderr = os.Stderr;
							err := cmd.Run ();
							if err != nil {
								fmt.Printf("-- metadata run: %v\n", err);
							} else {
								fmt.Printf("-- metadata run ok (%s)\n", tdelta(ts));
							}
						}

						dirs = make(map[string]bool);
					}
					time.Sleep (2 * time.Second);
					// fmt.Printf("wake\n");
					flag = false;
				}
			}
		}
	} ();

	mux := http.NewServeMux()
	flag.Parse()
	pth, err := filepath.Abs(*docroot)
	if (err != nil) {
		fmt.Printf("filepath.Abs(%v): %v\n",*docroot,err)
		return
	}

	mux.HandleFunc("/", root_handle_func(&opts{UrlPrefix: *urlprefix, Path: pth, IpHeader: *iphead}))

	log.Fatal(http.ListenAndServe(*addr, mux))
}

var counter uint64 = t64();

func root_handle_func(opts *opts) func (w http.ResponseWriter, r *http.Request) {
	phys := func (p string) string {
		return filepath.Join(opts.Path, filepath.FromSlash(p[len(opts.UrlPrefix):]))
	}

	return func (w http.ResponseWriter, r *http.Request) {
		if opts.IpHeader != "" {
			ff := r.Header.Get(opts.IpHeader)
			if ff != "" {
				r.RemoteAddr = ff
			}
		}

		fmt.Printf("%s %v %s \"%s\" \"%s\"\n", timestamp(), r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent());

		urlpath := path.Clean(r.URL.Path)

		tmp := urlpath;
		dirs := []string{};
		for {
			dir := path.Dir(tmp)	
			// fmt.Printf("dir '%s'\n", dir);
			if dir == "." || dir == "/" {
				http.Error(w, "bad path: " + urlpath, 400);
				return;
			}
			if dir == opts.UrlPrefix {
				break;
			}
			dirs = append(dirs,dir);
			tmp = dir;
		}
		// fmt.Printf("dirs.size: %d\n", len(dirs));
		file := phys(urlpath)

		if r.Method == "GET" || r.Method == "HEAD" {
			http.ServeFile(w, r, file)

		} else if r.Method == "PUT" {

			if len(dirs) < 1 {
				http.Error(w, "files must go inside repos: " + urlpath, 400);
				return;
			}

			st, err := os.Stat(phys(dirs[len(dirs)-1]));
			if os.IsNotExist(err) || (err == nil && !st.IsDir()) {
				http.Error(w, "won't create new repos by path: " + urlpath, 400);
				return;
			} else if err != nil {
				fmt.Printf("repo stat error:%v\n", err);
				http.Error(w, "internal error", 400);
				return;
			}

			ext := path.Ext(urlpath)
			if ext != ".rpm" {
				http.Error(w, "won't take non-.rpm: " + urlpath, 400);
				return;
			}

			fparent := dirs[0];
			err = os.MkdirAll(phys(fparent), 0777);
			if err != nil {
				http.Error(w, "failed to create dir: " + fparent, 400);
				return;
			}

			tmpfile := phys(fmt.Sprintf(".tmp%v", atomic.AddUint64(&counter, 1)));
			fmt.Printf("tmpfile: %s\n", tmpfile);

			rb := make([]byte, 65536);
			var total int64;
			var ts = t64();
			fo, err := os.Create (tmpfile);
			if err != nil {
				http.Error(w, "file create failure: " + err.Error(), 400);
				return;
			}
			defer os.Remove(tmpfile);
			defer fo.Close();
			for {
				n, err := r.Body.Read(rb);
				// fmt.Printf("n:%v\n", n);
				total += int64(n);
				_, errr := fo.Write(rb[0:n]); // _ is guaranteed to be the slice size unless errr is set.
				if errr != nil {
					fmt.Printf("write error:%v\n", errr);
					http.Error(w, "write error: " + errr.Error(), 400);
					return;
				}
				if err == io.EOF {
					// fmt.Printf("eof\n");
					break;
				}
				if err != nil {
					fmt.Printf("error:%v\n", err);
					http.Error(w, "internal error", 400);
					return;
				}
			}
			if total != r.ContentLength {
				fmt.Printf("content-length mismatch %d vs. %d\n", total, r.ContentLength);
			} else {
				fmt.Printf("total %d (%s)\n", total, tdelta(ts));
			}
			err = fo.Close();
			if err != nil {
				fmt.Printf("close error:%v\n", err);
				http.Error(w, "close error: " + err.Error(), 400);
				return;
			}
			err = os.Rename(tmpfile, file);
			if err != nil {
				fmt.Printf("rename error:%v\n", err);
				http.Error(w, "rename error: " + err.Error(), 400);
				return;
			}
			http.Error(w, "ok", 200);
			notifier <- phys(dirs[len(dirs)-1]);
			return;
		} else {
			http.Error(w, "method not supported", 400);
		}

	}
}
