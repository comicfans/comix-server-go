// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	//"github.com/goware/urlx"
	"archive/zip"
	"container/ring"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var imageExt = []string{"jpg", "gif", "png", "tif", "bmp", "jpeg", "tiff"}
var hiddenFullname = []string{".", "..", "@eaDir", "Thunmbs.db", ".DS_Store"}
var hiddenPartname = []string{"__MACOSX"}

type AppContext struct {
	root string
	//how many picture in zip will be preloaded ( forward = backword = preloadNumber)
	//0 disable, -1 unlimit
	preloadNumber int
	memLimit      uint64
	globalCache   *GlobalCache
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

	for e := range res {

		fmt.Fprintln(w, e)
	}

}

func isInZip(relpath string, ext string) bool {
	lowpath := strings.ToLower(relpath)

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

func processImage(w http.ResponseWriter, fullpath string, typestr string) {

	w.Header().Add("Content-Type", typestr)

	f, err := os.Open(fullpath)
	if err != nil {
		log.Println("error opening " + fullpath)
		return
	}

	defer f.Close()

	fi, errr := f.Stat()
	if errr != nil {
		log.Println("error stat " + fullpath)
		return
	}

	w.Header().Add("Content-Length", strconv.FormatInt(fi.Size(), 10))
	n, er := io.Copy(w, f)
	if er != nil {
		log.Println("error reading " + fullpath)
		return
	}
	log.Println("reading " + strconv.FormatInt(n, 10) + " bytes")
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
		processImage(w, fullpath, typestr)
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

type BaseCache struct {
	relpath  string
	memUsage uint64
}

type DirCache struct {
	BaseCache
	filelist []string
}

/**
* @brief cache file in zip
 */
type FileCache struct {
	BaseCache
	typestr string
	memory  []byte
}

type GlobalCache struct {
	rwMutex  sync.RWMutex
	cacheMap map[string]CacheResult
	lru      map[string]int
	memUsage uint64
	zipMap   map[string][]string
}

func mbString(byteNumber uint64) string {

	switch {
	case byteNumber < 1024*1024:
		return strconv.FormatFloat(float64(byteNumber)/1024, 'f', 2, 32) + "KB"
	case byteNumber < 1024*1024*1024:
		return strconv.FormatFloat(float64(byteNumber)/1024/1024, 'f', 2, 32) + "MB"
	default:
		return strconv.FormatFloat(float64(byteNumber)/1024/1024/1024, 'f', 2, 32) + "GB"
	}

}

func (ctx *AppContext) adjustCache() {

	g := ctx.globalCache
	g.rwMutex.RLock()
	if g.memUsage < ctx.memLimit {

		log.Println("cache size ", mbString(g.memUsage),
			" not exclude limit ", mbString(ctx.memLimit))
		g.rwMutex.RUnlock()
		return
	}

	g.rwMutex.RUnlock()

	g.rwMutex.Lock()
	defer g.rwMutex.Unlock()
	if g.memUsage < ctx.memLimit {
		//double check
		return
	}
	//TODO select from LRU
	log.Println("not implements")

}

func dirList(fullpath string) []string {

	ret := []string{}
	walkfunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path != fullpath {
			ret = append(ret, path)
		}

		return nil
	}

	filepath.Walk(fullpath, walkfunc)

	return ret

}

type FileInZip struct {
	file zip.File
	/**
	 * relpath to root
	 */
	relpath string
	typestr string
}

func readZipCache(fullpath string, preloadNumber int) (
	cacheResult CacheResult, preloadLis *ring.Ring, r *zip.ReadCloser) {

	reader, err := zip.OpenReader(fullpath)
	if err != nil {
		log.Println(err)
		return nil, nil, nil
	}

	cur := ring.New(preloadNumber)

	cacheResult = &DirCache{}

	for _, f := range reader.File {

		fi := f.FileInfo()

		cur.Value = zipfilepath + "/" + fi.Name()
		cur = cur.Next()

		if fi.Name() == imagepath {

			preloadList = cur.Prev()

		}
	}
}

func readInfoInZip(fullpath string, relpath string, preloadNumber int) (
	preloadList *ring.Ring, r *zip.ReadCloser, zipfilepath string) {

	zippos := strings.Index(fullpath, "zip")
	if zippos == -1 {
		zippos = strings.Index(fullpath, "cbz")
	}

	zipfilepath = fullpath[0 : zippos+3]

	log.Println("zip file path:" + zipfilepath)

	imagepath := strings.Replace(fullpath, zipfilepath+"/", "", -1)

	log.Println("image path :" + imagepath)

	reader, err := zip.OpenReader(zipfilepath)
	if err != nil {
		log.Println(err)
		return
	}

	cur := ring.New(preloadNumber*2 + 1)

	for _, f := range reader.File {

		fi := f.FileInfo()

		cur.Value = zipfilepath + "/" + fi.Name()
		cur = cur.Next()

		if fi.Name() == imagepath {

			preloadList = cur.Prev()

		}
	}

	return
}

func loadSingleFileInZip(cache *GlobalCache, fiz FileInZip, zipPath string, wg *sync.WaitGroup) CacheResult {

	defer wg.Done()

	cache.rwMutex.RLock()
	if v, ok := cache.cacheMap[fiz.relpath]; ok {
		cache.rwMutex.RUnlock()
		return v
	}
	cache.rwMutex.RUnlock()

	rc, err := fiz.file.Open()
	if err != nil {
		log.Println(err)
	}
	defer rc.Close()

	fc := &FileCache{
		BaseCache: BaseCache{
			relpath: fiz.relpath,
		},
		typestr: fiz.typestr,
		memory:  make([]byte, int(fiz.file.FileInfo().Size())),
	}

	_, err = io.ReadFull(rc, fc.memory)
	if err != nil {
		log.Println(err)
		return nil
	}

	fc.memUsage = uint64(len(fc.memory)) + uint64(len(fc.relpath))

	cache.rwMutex.Lock()
	defer cache.rwMutex.Unlock()

	//must double check ,or we may add it in another thread
	//then calc wrong memUsage
	if v, ok := cache.cacheMap[fiz.relpath]; ok {
		return v
	}

	cache.cacheMap[fiz.relpath] = fc
	cache.memUsage += fc.memUsage

	return fc
}

func readerCloseRoute(wg *sync.WaitGroup, rc *zip.ReadCloser) {

	wg.Wait()
	rc.Close()
}

func loadMultiFilesInZip(globalCache *GlobalCache, preloadList *ring.Ring, zipPath string, rc *ReadCloser) {

	wg := &sync.WaitGroup{}
	//preload in background
	for i := preloadList.Next(); i.Value != nil; i = preloadList.Next() {

		wg.Add(1)
		go loadSingleFileInZip(globalCache, i.Value.(FileInZip), zipPath, wg)
	}

	go readerCloseRoute(wg, rc)
}

func loadToCache(ctx *AppContext, relToRoot string, abspath string) (
	CacheResult, bool, string) {

	if isDir(abspath) {

		ret := &DirCache{
			BaseCache: BaseCache{
				relpath: relToRoot,
			},
			filelist: dirList(abspath),
		}

		go addToCache(ctx.globalCache, relToRoot, ret)

		return ret, false, ""
	}

	ext := filepath.Ext(relToRoot)

	if len(ext) >= 1 {

		ext = ext[1:]
	}

	log.Printf("ext of %s is %s", relToRoot, ext)

	if !inArray(ext, imageExt) {
		//not a image file
		return nil, true, ""
	}

	typestr := getContentType(ext)

	log.Printf("type of %s is %s", relToRoot, typestr)

	if isInZip(relToRoot, ext) {

		preloadList, rc, zipPath := readInfoInZip(abspath, relToRoot, ctx.preloadNumber)

		if preloadList.Value == nil {
			//not found in file
			return nil, true, typestr
		}

		currentRequested := loadSingleFileInZip(ctx.globalCache,
			preloadList.Value.(FileInZip), zipPath, nil)

		loadMultiFilesInZip(preloadList.Next(), zipPath, rc)

		return currentRequested, false, ""
	}

	if ext == "zip" || ext == "cbz" {
		readZipCache()
		return
	}
	//is image file
	//TODO
	return nil, false, typestr
}

func addToCache(g *GlobalCache, relToRoot string, cache CacheResult) {

	g.rwMutex.Lock()
	defer g.rwMutex.Unlock()

	if _, ok := g.cacheMap[relToRoot]; ok {
		//already added by other thread
		return
	}

	//TODO update LRU
	g.cacheMap[relToRoot] = cache

	g.memUsage += cache.MemUsage()

	if fc := cache.(*FileCache); fc != nil {
		append(g.zipMap[relToRoot], "")
	}

	log.Println("global cache size ", mbString(g.memUsage))
}

func cacheViewHandler(
	ctx *AppContext, w http.ResponseWriter, r *http.Request) {

	g := ctx.globalCache

	relToRoot := r.URL.Path

	//TODO trim leading slash

	g.rwMutex.RLock()

	if val, ok := g.cacheMap[relToRoot]; ok {

		log.Println("found cache for ", relToRoot)
		//TODO update LRU
		g.rwMutex.RUnlock()
		val.Process(w)

		go ctx.adjustCache()
		return
	}

	g.rwMutex.RUnlock()

	abspath := ctx.root + "/" + relToRoot

	loaded, hasError, typestr := loadToCache(ctx, relToRoot, abspath)

	if hasError {
		//not in cache or not image
		return
	}

	if !hasError && loaded != nil {
		loaded.Process(w)
		go ctx.adjustCache()
		return
	}

	//no error but not dircache nor filecache, is image file
	processImage(w, abspath, typestr)
}

type CacheResult interface {
	Process(http.ResponseWriter)
	MemUsage() uint64
}

func (b *BaseCache) MemUsage() uint64 {
	return b.memUsage
}

func (f *FileCache) Process(w http.ResponseWriter) {

	w.Header().Add("Content-Type", f.typestr)
	w.Header().Add("Content-Length", strconv.FormatInt(int64(len(f.memory)), 10))
	w.Write(f.memory)
}

func (d *DirCache) Process(w http.ResponseWriter) {

	w.Header().Set("Content-Type", "text/plain")

	for _, name := range d.filelist {
		fmt.Fprintln(w, name)
	}

}

func main() {

	log.SetOutput(os.Stdout)

	ip := flag.String("addr", "0.0.0.0", "listen on address, default listen on all interfaces")

	port := flag.Uint("port", 31257, "listen port ")

	preloadNumber := flag.Int("preload", 10, "preload number in zip file (both forward and backword), 0 to disable , -1 unlimit preload")

	limitMB := flag.Uint("limit", 256, "max cache size (in MB) ")

	root := flag.String("root", "root", "root path of mangas")

	flag.Parse()

	abspath := *root

	if !filepath.IsAbs(*root) {
		cwd, _ := os.Getwd()
		abspath = cwd + string(filepath.Separator) + *root
	}

	abspath = path.Clean(abspath)

	log.Println("use " + abspath + "as root dir")

	ipport := *ip + ":" + strconv.Itoa(int(*port))

	log.Println("try to listen on " + ipport)

	appContext := AppContext{
		root:          abspath,
		memLimit:      uint64(*limitMB) * 1024 * 1024,
		preloadNumber: *preloadNumber,
		globalCache:   &GlobalCache{},
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
