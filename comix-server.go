// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	//"github.com/goware/urlx"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
)

type AppContext struct {
	root string
}

type ContextHandler interface {
	ServeHTTPContext(*AppContext, http.ResponseWriter, *http.Request)
}

type ContextHandlerFunc func(*AppContext, http.ResponseWriter, *http.Request)

func (h ContextHandlerFunc) ServeHTTPContext(ctx *AppContext, rw http.ResponseWriter, req *http.Request) {
	h(ctx, rw, req)
}

func listDir(relPath string, w http.ResponseWriter) {

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "jjjjjjjjjdas")
}

func viewHandler(ctx *AppContext, w http.ResponseWriter, r *http.Request) {
	log.Println("request is '" + r.URL.String() + "', after normailze :'")

	fullpath := path.Join(ctx.root, r.URL.Path)

	//listDir(r.URL.Path[len("/"):], w)
	fmt.Fprintf(w, "url path %s, after join: %s", r.URL.Path, fullpath)
}

type ContextAdapter struct {
	ctx     *AppContext
	handler ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ca.handler.ServeHTTPContext(ca.ctx, rw, req)
}

func main() {

	log.SetOutput(os.Stdout)

	ip := flag.String("addr", "0.0.0.0", "listen on address,default 0.0.0.0 (all interface)")

	port := flag.Uint("port", 31257, "listen port , default 31257")

	root := flag.String("", "", "root path of manga, default to 'root' directory of current working dir ")

	flag.Parse()

	if len(*root) == 0 {
		wd, _ := os.Getwd()
		*root = path.Join(wd, "root")
		log.Println("use " + *root + " as root ")
	}

	if net.ParseIP(*ip) == nil {
		log.Fatalln("invaid addr " + *ip)
		return
	}

	ipport := *ip + ":" + strconv.Itoa(int(*port))

	log.Println("try to listen on " + ipport)

	appContext := AppContext{
		root: *root,
	}

	s := &ContextAdapter{
		ctx:     &appContext,
		handler: ContextHandlerFunc(viewHandler),
	}

	err := http.ListenAndServe(ipport, s)
	if err != nil {
		log.Fatalln("can not listen on " + ipport)
	}

}
