// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	//"github.com/goware/urlx"
	"archive/zip"
	"sync"
	"io"
	"container/list"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var imageExt = []string{"jpg", "gif", "png", "tif", "bmp", "jpeg", "tiff"}
var hiddenFullname = []string{".", "..", "@eaDir", "Thunmbs.db", ".DS_Store"}
var hiddenPartname = []string{"__MACOSX"}

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

func listDir(fullpath string, w http.ResponseWriter) {

	log.Println("list dir " + fullpath)

	w.Header().Set("Content-Type", "text/plain")

    res := dirList(fullpath)

    for _,f := range res{
        fmt.Fprintln(w, f)
    }


}

func isInZip(path string, ext string) bool {
	lowpath := strings.ToLower(path)

	if strings.Index(lowpath, "zip") == -1 && strings.Index(lowpath, "cbz") == -1 {
		return false
	}

	return ext != "zip" && ext != "cbz"

}

func processFileInZip(appcontex *AppContext,
	w http.ResponseWriter, fullpath string, typestr string) {

	zippos := strings.Index(fullpath, "zip")
	if zippos == -1 {
		zippos = strings.Index(fullpath, "cbz")
	}

	zipfilepath := fullpath[0 : zippos+3]

	log.Println("zip file path:" + zipfilepath)

	imagepath := strings.Replace(fullpath, zipfilepath+"/", "", -1)

	log.Println("image path :" + imagepath)

	reader, err := zip.OpenReader(zipfilepath)
	if err != nil {
		log.Println(err)
		return
	}

	defer reader.Close()

	for _, f := range reader.File {

		fi := f.FileInfo()

		if fi.Name() == imagepath {

			log.Println("found image " + imagepath)
			w.Header().Add("Content-Type", typestr)
			w.Header().Add("Content-Length", strconv.FormatInt(fi.Size(), 10))
			rc, err := f.Open()
			if err != nil {
				log.Println(err)
			}
			defer rc.Close()

			_, err = io.Copy(w, rc)
			if err != nil {
				log.Println(err)
			}

		}

	}

}

func processImage(appcontex *AppContext,
	w http.ResponseWriter, path string, imageExt string) {
}

func isSupport(filename string, isdir bool) bool {

	if filename[0:1] == "." {
		return false
	}

	if inArray(filename, hiddenFullname) {
		return false
	}

	for _, p := range hiddenPartname {
		if strings.Contains(filename, p) {

		}
	}

	return true
}

func processZip(appcontex *AppContext,
	w http.ResponseWriter, fullpath string) {

	log.Printf("process zip %s\n", fullpath)

	if fullpath[len(fullpath)-1:] == "/" {
		fullpath = fullpath[0 : len(fullpath)-1]
		log.Printf("as %s\n", fullpath)
	}

	reader, err := zip.OpenReader(fullpath)
	if err != nil {
		log.Println(err)
		return
	}

	defer reader.Close()

	for _, f := range reader.File {

		fi := f.FileInfo()
		if isSupport(fi.Name(), fi.IsDir()) {

			fmt.Fprintln(w, fi.Name())
		}

	}

}

func getContentType(ext string) string {
	switch ext {
	case "jpg":
		return "image/jpeg"
	case "tif":
		return "image/tiff"
	default:
		return "image/" + "ext"
	}
}

func inArray(check string, array []string) bool {

	for _, inarray := range array {
		if check == inarray {
			return true
		}
	}
	return false
}

func isDir(fullpath string) bool {

	file, err := os.Open(fullpath)
	if err != nil {

		log.Println(err)
		return false
	}
	defer file.Close()

	fileinfo, err := file.Stat()
	if err != nil {
		log.Fatalln(err)
		return false
	}

	return fileinfo.IsDir()

}

func viewHandler(ctx *AppContext, w http.ResponseWriter, r *http.Request) {
	log.Println("request is '" + r.URL.String())

	fullpath := path.Join(ctx.root, r.URL.Path)

	//listDir(r.URL.Path[len("/"):], w)
	log.Printf("url path %s, after join: %s\n", r.URL.Path, fullpath)

	if isDir(fullpath) {
		log.Printf("%s is dir \n", fullpath)
		listDir(fullpath, w)
		return
	}

	log.Printf("%s is not dir \n", fullpath)

	ext := filepath.Ext(fullpath)
	if len(ext) >= 1 {

		ext = ext[1:]
	}

	log.Printf("ext of %s is %s", fullpath, ext)

	typestr := getContentType(ext)

	log.Printf("type of %s is %s", fullpath, typestr)

	if isInZip(fullpath, ext) {
		processFileInZip(ctx, w, fullpath, typestr)
		return
	}

	if inArray(ext, imageExt) {
		processImage(ctx, w, fullpath, typestr)
		return
	}

	if ext == "zip" || ext == "cbz" {
		processZip(ctx, w, fullpath)
		return
	}

}

type ContextAdapter struct {
	ctx     *AppContext
	handler ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ca.handler.ServeHTTPContext(ca.ctx, rw, req)
}


type DirCache struct{
    abspath string
    filelist *List
    memUsage int
}

/**
* @brief cache file in zip
*/
type FileCache struct{
    abspath string
    typestr string
    memory []byte
}

type GlobalCache struct{
    rwMutex sync.RWMutex
    cacheMap map[string]CacheResult
    lru map[string]int
}

func dirList(path string) container.List{

    ret := list.New
    walkfunc := func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if path != fullpath {
            ret.PushBack(path)
        }

        return nil
    }

    filepath.Walk(fullpath, walkfunc)

    return ret

}

type FileInZip struct{
    file File 
    fullname string
}



func readInfoInZip(fullpath string, preloadBid int) (
    bidList *Ring,r ReaderCloser){

    zippos := strings.Index(fullpath, "zip")
	if zippos == -1 {
		zippos = strings.Index(fullpath, "cbz")
	}

	zipfilepath := fullpath[0 : zippos+3]

	log.Println("zip file path:" + zipfilepath)

	imagepath := strings.Replace(fullpath, zipfilepath+"/", "", -1)

	log.Println("image path :" + imagepath)

	reader, err := zip.OpenReader(zipfilepath)
	if err != nil {
		log.Println(err)
		return
	}

    ret:=ring.New()

    found := false

    var toRead *FileInZip =nil
	for _, f := range reader.File {

		fi := f.FileInfo()

		if fi.Name() == imagepath {

            found=true
            idx = bidList.Len()-1

        }else{

            if bidList.Len() > preloadBid*2 {
                break
            }

            if !found && bidList.Len()>preloadBid{
                bidList.Remove(bidList.Front())
            }
        }
	}

    if found {
        reorderPreloadList(bidList)
    }
}

func loadFileInZip(wg *sync.WaitGroup,cache *GlobalCache,fiz FileInZip) *CacheResult{

    defer wg.Done()

    cache.rwMutex.RLock()
    if v,ok:=cache.cacheMap[f.fullname]{
        cache.rwMutex.RUnlock()
        return v
    }
    cache.rwMutex.RUnlock()

    rc, err := fiz.file.Open()
    if err != nil {
        log.Println(err)
    }
    defer rc.Close()

    _, err = io.Copy(w, rc)
    if err != nil {
        log.Println(err)
    }

    fc := FileCache{
        fullpath  fiz.fullpath
        typestr fiz.typestr
        memory io.Read(rc)
    }

    fc.memUsage = Len(fc.memory) + Len(fc.fullpath)

    cache.rwMutex.Lock()
    cache.cacheMap [fullpath] = fc

    defer cache.rwMutex.UnLock()

    return fc

}

func readerCloseRoute(wg sync.WaitGroup,rc ReaderCloser){

}

func loadAsCache (path string ) *CacheResult{

    if isDir(path) {

        ret = new DirCache{
            abspath path
            filelist dirList()
        }

        go addToCache(ret)

        return ret
    }

    ext := filepath.Ext(fullpath)

	if len(ext) >= 1 {

		ext = ext[1:]
	}

	log.Printf("ext of %s is %s", fullpath, ext)

	typestr := getContentType(ext)

	log.Printf("type of %s is %s", fullpath, typestr)

	if isInZip(fullpath, ext) {

        preloadList := readInfoInZip(fullpath)

        if preloadList ==nil {
            return 
        }

        current := preloadList.Value()

        currentRequested := loadFileInZip(nil,current)

        wg := sync.WaitGroup 
        //preload in background
        for i:= preloadList.Next();i!=nil;i=preload(){

            wg.Add(1);
            go loadFileInZip(&wg,i)
        }

        go readerCloseRoute(wg)

        return currentRequested
    }
}

func addToCache(path string,cache *CacheResult){


    g.rwMutex.Lock()
    defer g.rwMutex.Unlock()

    if val,ok:=g.cacheMap ; ok{
        //already added by other thread

        return
    }

    g.cacheMap[path] = cache
}

func (g *GlobalCache) load (path string) *CacheResult{

    g.rwMutex.RLock()

    if val,ok:=g.cacheMap[path] ;ok{
        g.rwMutex.RUnlock()
        return val
    }

    g.rwMutex.RUnlock()
    loaded := loadAsCache(path)

    if loaded == nil {
       return nil 
    }

    return loaded
}

type CacheResult interface {

    MemUsage()int
}

func (f *FileCache) Process(ctx *AppContext,w http.ResponseWriter){

    w.Header().Add("Content",f.typestr);
    w.Write(f.memory);
}

func (d *DirCache) Process(ctx *AppContext,w http.ResponseWriter){

    w.Header().Set("Content-Type", "text/plain")

    for _,name:= range d.filelist {
        fmt.Fprintln(w, name)
    }

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
