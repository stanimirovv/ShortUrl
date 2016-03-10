package main

import  (
            "github.com/golang/glog"
            "flag"
            "net/http"
            "os"

            "github.com/gorilla/mux"
            _ "github.com/go-sql-driver/mysql"
            "io/ioutil"
            "net/url"
            "math/rand"
            "time"
            "errors"
        )


var cache *Cache // In memory cache

func main() {
    //Initialize glog
    flag.Parse()

    //Initialize cache
    cache = NewCache(10)

    //Initialize the DB
    InitStorage()

    //Used in url hash generation 
    rand.Seed(time.Now().UTC().UnixNano())

    //File server
    go http.ListenAndServe(":" + os.Getenv("MICRO_URL_FILE_PORT"), http.FileServer(http.Dir(os.Getenv("MICRO_URL_FILES_DIR"))))
    glog.Info("Started the file server on port:" + os.Getenv("MICRO_URL_FILES_DIR"))

    //Initialize the REST API routes
    router := mux.NewRouter().StrictSlash(false)
    router.HandleFunc("/g/{urlHash}", Redirect)
    router.HandleFunc("/check/{urlHash}", Check)
    router.HandleFunc("/add/", Add)
    router.HandleFunc("/delete/{urlHash}", Remove)

    glog.Info("Starting the API server on port:" + os.Getenv("MICRO_URL_API_PORT"))
    glog.Info(http.ListenAndServe(":" + os.Getenv("MICRO_URL_API_PORT"), router))

}


/* Handlers  */
func Redirect(w http.ResponseWriter, r *http.Request){
    if r.Method != "GET" {
        WriteResp(w, http.StatusMethodNotAllowed, `Wrong method!`)
        return
    }

    vars := mux.Vars(r)

    //Check if URL is in cache
    cachedUrl := cache.GetVal(vars[`urlHash`])
    if cachedUrl != `` {
        glog.Info(`Redirecting (from cache) with hash: `, vars[`urlHash`], ` to: `, cachedUrl)
        http.Redirect(w, r, cachedUrl, http.StatusMovedPermanently)
        return
    }

    //URL is not in cache, fetch it and cache it
    url, err := GetURLFromStorage(vars[`urlHash`])
    if err != nil {
        WriteResp(w, http.StatusInternalServerError, `Please try again later!`)
    }
    glog.Info(`Redirecting with hash: `, vars[`urlHash`], ` to: `, url)
    cache.Add(vars[`urlHash`], url)

    if url == `` {
        WriteResp(w, http.StatusNotFound, `Not found!`)
        return
    }

    http.Redirect(w, r, url, http.StatusMovedPermanently)
}

func Check(w http.ResponseWriter, r *http.Request){
    if r.Method != "GET" {
        WriteResp(w, http.StatusMethodNotAllowed, `Wrong method!`)
        return
    }

    vars := mux.Vars(r)
    url, err := GetURLFromStorage(vars[`urlHash`])
    if err != nil {
        WriteResp(w, http.StatusInternalServerError, `Please try again later!`)
    }
    glog.Info(`Redirecting with hash: `, vars[`urlHash`], ` to: `, url)

    if url == `` {
        WriteResp(w, http.StatusNotFound, `Not found!`)
        return
    }


    WriteResp(w, http.StatusOK, url)
}


func Add(w http.ResponseWriter, r *http.Request){
    w.Header().Set("Access-Control-Allow-Origin", "*")

    if r.Method != "POST" {
        WriteResp(w, http.StatusMethodNotAllowed, `Wrong method!`)
        glog.Error(r.Method)
        return
    }

    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        WriteResp(w, http.StatusInternalServerError, `Please try again later!`)
    }
    bodyStr := string(body) //bytes to string
    glog.Info(bodyStr)
    if bodyStr == `` {
        WriteResp(w, http.StatusBadRequest, `Invalid URL!`)
        return
    }

    err = checkUrl(bodyStr)
    if err != nil {
        WriteResp(w, http.StatusBadRequest, `Invalid URL!`)
        glog.Error(err, ` Invalid URL:  `, bodyStr)
        return
    }

    urlHash  := GenerateHash(bodyStr, 8)
    glog.Error(`urlHash: `, urlHash)
    urlHash, err = AddURLToStorage(urlHash, bodyStr)
    if err != nil {
        WriteResp(w, http.StatusInternalServerError, `Please try again later!`)
    }
    w.Write([]byte(urlHash))
}

func Remove(w http.ResponseWriter, r *http.Request){
    if r.Method != "DELETE"{
        WriteResp(w, http.StatusMethodNotAllowed, `Wrong method!`)
        return
    }

    vars := mux.Vars(r)
    err := DeleteURL(vars[`urlHash`])
    if err != nil {
        WriteResp(w, http.StatusInternalServerError, `Please try again later!`)
    }
    glog.Info(`Deleted hash: `, vars[`urlHash`])

    w.Write([]byte(`Deleted hash: ` + vars[`urlHash`]))
}

// HELPER FUNCTIONS
func WriteResp(w http.ResponseWriter, status int, msg string){
    http.Error(w, msg, status)
}

func GenerateHash(inp string, length int) string{
    const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
    b := make([]byte, length)
    for i := range b {
        b[i] = letterBytes[rand.Int63() % int64(len(letterBytes))]
    }
    return string(b)
}

func checkUrl(input string) error {

    u, err := url.Parse(input)
    if err != nil {
        glog.Error("  error:", err)
        return errors.New(`Not an URL!`)
    }

    if u.Scheme == "" {
        u, _ = url.Parse("http://" + input)
    } else if u.Scheme != "http" && u.Scheme != "https" {
        glog.Error("  error: scheme '%s' unsupported\n", u.Scheme)
        return errors.New(`Not an URL!`)
    }

/*
    if os.Getenv(`URL_CHECK_STRICT_MODE`) == `true` {
        _, err := net.LookupHost(input)

        if err != nil {
            glog.Error(err, ` Invalid URL:  `, input)
            return errors.New(`URL was not resolved!`)
        }
    }
*/
    return nil
}
