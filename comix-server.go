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
	"io/ioutil"
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
	rank     uint64
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
	memory []byte
}

type RankPair struct {
	rank    uint64
	relpath string
}

type GlobalCache struct {
	rwMutex   sync.RWMutex
	cacheMap  map[string]CacheResult
	memUsage  uint64
	rankMap   map[uint64]string
	belongZip map[string][]string
	rank      uint64
	lru       []uint64
}

func humanSizeString(byteNumber uint64) string {

	switch {
	case byteNumber < 1024:
		return strconv.FormatInt(int64(byteNumber), 10) + "B"
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

		log.Println("cache size ", humanSizeString(g.memUsage),
			" not exclude limit ", humanSizeString(ctx.memLimit))
		g.rwMutex.RUnlock()
		return
	}

	g.rwMutex.RUnlock()

	g.rwMutex.Lock()
	defer g.rwMutex.Unlock()

	for g.memUsage > ctx.memLimit {
		removeRank := g.lru[0]
		g.lru = g.lru[1:]
		log.Println("choose rank ", removeRank, " to remove")
		// this rank may be useless (updated)
		relpath, hasKey := g.rankMap[removeRank]

		if hasKey {
			removeCache, shouldBeTrue := g.cacheMap[relpath]

			if !shouldBeTrue {
				log.Println("bad logic ,rank ", removeRank, " didn't have respected relpath")
			} else {

				log.Println("remove cache rank:", removeRank, " relpath:", relpath, " size:", removeCache.MemUsage())
				delete(g.cacheMap, relpath)
				delete(g.rankMap, removeRank)

				g.memUsage -= removeCache.MemUsage()
			}
		} else {
			log.Println("rank ", removeRank, " already dailing")
		}

	}

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
	file *zip.File
	/**
	 * relpath to root
	 */
	relpath string
}

func readZipCache(relToRoot, fullpath string, preloadNumber int) (
	cacheResult CacheResult, preloadList *ring.Ring, r *zip.ReadCloser) {

	r, err := zip.OpenReader(fullpath)
	if err != nil {
		log.Println(err)
		return nil, nil, nil
	}

	//preload from head
	preloadList = ring.New(preloadNumber)

	dirCache := DirCache{
		BaseCache: BaseCache{
			relpath: relToRoot,
		},
	}

	for i, f := range r.File {

		fi := f.FileInfo()

		if !isSupport(fi.Name(), false) {
			continue
		}

		relToZip := fi.Name()

		relfilepath := relToRoot + "/" + relToZip

		if i < preloadNumber {

			log.Println("put to preloadlist ", relfilepath)

			fiz := &FileInZip{
				relpath: relfilepath,
				file:    f,
			}

			preloadList.Value = fiz
			preloadList = preloadList.Next()
		}

		dirCache.Append(relToZip)

	}

	log.Println("preloadList len ", preloadList.Len())

	cacheResult = &dirCache

	return
}

func readInfoInZip(rootPath string, relToRoot string, preloadNumber int) (
	preloadList *ring.Ring, r *zip.ReadCloser, zipfilepath string) {

	zippos := strings.Index(relToRoot, "zip")
	if zippos == -1 {
		zippos = strings.Index(relToRoot, "cbz")
	}

	zipRelPath := relToRoot[0 : zippos+3]

	zipfilepath = rootPath + string(filepath.Separator) + zipRelPath

	log.Println("zip file path:" + zipfilepath)

	imagepath := strings.Replace(relToRoot, zipRelPath+"/", "", -1)

	log.Println("image path :" + imagepath)

	r, err := zip.OpenReader(zipfilepath)
	if err != nil {
		log.Println(err)
		return
	}

	cur := ring.New(preloadNumber*2 + 1)

	for _, f := range r.File {

		fi := f.FileInfo()

		fiz := &FileInZip{
			file:    f,
			relpath: zipRelPath + "/" + fi.Name(),
		}

		cur.Value = fiz
		cur = cur.Next()

		if fi.Name() == imagepath {

			preloadList = cur.Prev()

		}

		if preloadList != nil && preloadList.Len() > preloadNumber*2 {
			return
		}
	}

	return
}

func loadSingleFileInZip(cache *GlobalCache, fiz *FileInZip, zipPath string, wg *sync.WaitGroup) CacheResult {

	log.Println("load ", fiz.file.FileInfo().Name(), " in ", zipPath)

	if wg != nil {
		defer wg.Done()
	}

	cache.rwMutex.RLock()
	//early check if already in cache ,avoid useless read
	if c, added := cache.cacheMap[fiz.relpath]; added {
		cache.rwMutex.RUnlock()
		cache.rwMutex.Lock()
		defer cache.rwMutex.Unlock()
		updateCacheRankUnlocked(cache, c.(*FileCache))
		return c
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
		memory: make([]byte, int(fiz.file.FileInfo().Size())),
	}

	_, err = io.ReadFull(rc, fc.memory)
	if err != nil {
		log.Println(err)
		return nil
	}

	fc.memUsage = uint64(len(fc.memory)) + uint64(len(fc.relpath))

	//when load in concurrent, is by preload ,not update rank if clash
	//(or we may update too many by lots preload)
	//when not load in concurrent, is by request, update rank if clash
	go addToCache(cache, fc, zipPath, wg == nil)

	return fc
}

func readerCloseRoute(wg *sync.WaitGroup, rc *zip.ReadCloser) {

	wg.Wait()
	rc.Close()
}

func loadMultiFilesInZip(globalCache *GlobalCache, preloadList *ring.Ring,
	exclude interface{}, zipPath string, rc *zip.ReadCloser) {

	wg := &sync.WaitGroup{}
	//preload in background

	preloadList.Do(func(i interface{}) {

		if i != exclude && i != nil {
			wg.Add(1)
			cast := i.(*FileInZip)
			log.Println(" concurrent load ", cast.file.FileInfo().Name())
			go loadSingleFileInZip(globalCache, cast, zipPath, wg)
		}
	})

	go readerCloseRoute(wg, rc)
}

func (d *DirCache) Append(path string) {

	log.Println("append ", path, " to ", d.relpath)

	d.filelist = append(d.filelist, path)

	d.memUsage += uint64(len(path))
}

func readDirCache(abspath, relToRoot string) *DirCache {

	dc := &DirCache{
		BaseCache: BaseCache{
			relpath: relToRoot,
		},
	}

	files, err := ioutil.ReadDir(abspath)

	if err != nil {
		log.Println(err)
		return nil
	}

	for _, f := range files {
		dc.Append(f.Name())
	}

	return dc
}

func tryCacheOrLoad(ctx *AppContext, relToRoot string, abspath string) (
	loadedCache CacheResult, hasError bool, typestr string) {

	if isDir(abspath) {

		loadedCache = readDirCache(abspath, relToRoot)

		if loadedCache == nil {
			hasError = true
			typestr = ""
			return
		}

		go addToCache(ctx.globalCache, loadedCache, "", false)

		return loadedCache, false, ""
	}

	ext := filepath.Ext(relToRoot)

	if len(ext) >= 1 {

		ext = ext[1:]
	}

	log.Printf("ext of %s is %s", relToRoot, ext)

	if ext == "zip" || ext == "cbz" {

		log.Println(relToRoot, " is zip file")

		cache, preloadList, rc := readZipCache(relToRoot, abspath, ctx.preloadNumber)

		log.Println("len of preload list ", preloadList.Len())

		if cache == nil {
			return nil, true, ""
		}

		go addToCache(ctx.globalCache, cache, "", false)

		go loadMultiFilesInZip(ctx.globalCache, preloadList, nil, relToRoot, rc)

		return cache, false, ""
	}

	//not zip file

	if !inArray(ext, imageExt) {
		//not a image file
		log.Println(relToRoot, " is not image file")
		return nil, true, ""
	}

	typestr = getContentType(ext)

	log.Printf("type of %s is %s", relToRoot, typestr)

	if isInZip(relToRoot, ext) {

		preloadList, rc, zipPath := readInfoInZip(ctx.root, relToRoot, ctx.preloadNumber)

		if preloadList == nil {
			//not found in file
			return nil, true, typestr
		}

		cast := preloadList.Value.(*FileInZip)
		log.Println("load ", relToRoot, " in current goroute")
		currentRequested := loadSingleFileInZip(ctx.globalCache,
			cast, zipPath, nil)

		loadMultiFilesInZip(ctx.globalCache, preloadList, preloadList.Value, zipPath, rc)

		return currentRequested, false, ""
	}

	//is image file
	return nil, false, typestr
}

func (b *BaseCache) String() string {
	return fmt.Sprint("relpath:'", b.relpath, "' memUsage:", humanSizeString(b.memUsage))
}

func (f *FileCache) String() string {
	return f.BaseCache.String() + fmt.Sprint(" rank:", f.rank)
}

func updateCacheRankUnlocked(g *GlobalCache, fc *FileCache) {

	oldRank := fc.rank

	//update rank
	delete(g.rankMap, oldRank)

	g.rank++

	log.Println("update cache: old rank of cache ", fc.RelPath(), ":", oldRank, "will be updated to ", g.rank)
	g.rankMap[g.rank] = fc.RelPath()
	//we leave old rank as gabage in LRU, not care
	//since we already remove the rankMap key so
	//when adjust cache ,it will be removed from lru
	g.lru = append(g.lru, g.rank)
	fc.rank = g.rank
}

func addToCache(g *GlobalCache, cache CacheResult, belongToZip string, updateRank bool) {

	log.Println("add to cache with:", cache, " belong to zip ", belongToZip)

	g.rwMutex.Lock()
	defer g.rwMutex.Unlock()

	old, added := g.cacheMap[cache.RelPath()]
	if added && belongToZip /*cache lru only apply to FileCache */ != "" && updateRank {
		//already added by other thread
		updateCacheRankUnlocked(g, old.(*FileCache))
		return
	}

	g.rank++
	g.cacheMap[cache.RelPath()] = cache

	g.memUsage += cache.MemUsage()

	if belongToZip != "" {

		fc := cache.(*FileCache)
		fc.rank = g.rank

		g.belongZip[cache.RelPath()] = append(g.belongZip[cache.RelPath()], belongToZip)
		g.rankMap[g.rank] = cache.RelPath()
		g.lru = append(g.lru, g.rank)
	}

	log.Println("adding new cache :", cache)

	log.Println("global cache size ", humanSizeString(g.memUsage))
}

func updateRankLocked(g *GlobalCache, c CacheResult) {

	switch c.(type) {
	case *FileCache:
		g.rwMutex.Lock()
		updateCacheRankUnlocked(g, c.(*FileCache))
		g.rwMutex.Unlock()
	default:
		return
	}

}

func cacheViewHandler(
	ctx *AppContext, w http.ResponseWriter, r *http.Request) {

	g := ctx.globalCache

	relToRoot := r.URL.Path

	relToRoot = strings.TrimRight(strings.TrimLeft(relToRoot, "/"), "/")

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

	loaded, hasError, typestr := tryCacheOrLoad(ctx, relToRoot, abspath)

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
	RelPath() string
	Process(http.ResponseWriter)
	MemUsage() uint64
}

func (b *BaseCache) RelPath() string {
	return b.relpath
}

func (b *BaseCache) MemUsage() uint64 {
	return b.memUsage
}

func (f *FileCache) Process(w http.ResponseWriter) {

	ext := filepath.Ext(f.relpath)[1:]

	w.Header().Add("Content-Type", getContentType(ext))
	w.Header().Add("Content-Length", strconv.FormatInt(int64(len(f.memory)), 10))
	w.Write(f.memory)
}

func (d *DirCache) Process(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("charset", "UTF-8")

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

	appContext := &AppContext{
		root:          abspath,
		memLimit:      uint64(*limitMB) * 1024 * 1024,
		preloadNumber: *preloadNumber,
		globalCache: &GlobalCache{
			cacheMap:  make(map[string]CacheResult),
			belongZip: make(map[string][]string),
			rankMap:   make(map[uint64]string),
		},
	}

	log.Println("try to listen on ", ipport, " memlimit:", humanSizeString(appContext.memLimit), " use '", appContext.root, "' as root")

	s := &ContextAdapter{
		ctx:     appContext,
		handler: ContextHandlerFunc(cacheViewHandler),
	}

	err := http.ListenAndServe(ipport, s)
	if err != nil {
		log.Fatalln("can not listen on " + ipport)
	}

}
