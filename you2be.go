package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"github.com/gin-gonic/gin"
	// "reflect"
	"crypto/md5"
	"io"
	"log"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/go-redis/redis"
)

// URLData POST BODY JSON
type URLData struct {
	URL string `json:"url" binding:"required"`
}

// VideoData 视频地址信息
type VideoData struct {
	title    string
	videoURL string
	audioURL string
	length   int
}

// ConfigInfo 配置文件
type ConfigInfo struct {
	listenAddr string
	redisAddr  string
	savePath   string
	wwwPath    string
}

// DLserver 并行下载
type DLserver struct {
	WG    sync.WaitGroup
	Gonum chan string
}

// getConfig 读取配置文件
func getConfig() (configInfo *ConfigInfo) {
	cfg, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal("config.json Not Found")
	}
	configInfo = new(ConfigInfo)
	configJSON := make(map[string]interface{})
	json.Unmarshal(cfg, &configJSON)
	// fmt.Println(reflect.TypeOf(configJSON))
	configInfo.listenAddr = configJSON["listen_addr"].(string)
	configInfo.redisAddr = configJSON["redis_addr"].(string)
	configInfo.savePath = configJSON["save_path"].(string)
	configInfo.wwwPath = configJSON["www_path"].(string)
	return
}

// getCurrentDirectory 获取当前路径
func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

// maxInt 获取最大值
func maxInt(num []int) int {
	max := 0
	for _, n := range num {
		if n > max {
			max = n
		}
	}
	return max
}

// rangePart 计算文件分片区间
func rangePart(length int, part int) (ran []string) {
	offset := length / part
	for i := 0; i < part; i++ {
		if i == part-1 {
			iTmp := strconv.Itoa(i*offset) + "-" + strconv.Itoa(length)
			ran = append(ran, iTmp)
		} else {
			iTmp := strconv.Itoa(i*offset) + "-" + strconv.Itoa((i+1)*offset)
			ran = append(ran, iTmp)
		}
	}
	return
}

// pathJoin 路径拼接
func pathJoin(path []string) (jPath string) {
	ostype := runtime.GOOS
	if ostype == "windows" {
		jPath = strings.Join(path, "\\")
	} else if ostype == "linux" {
		jPath = strings.Join(path, "/")
	}
	return
}

// getYou2beRawData 获取 youtube 播放器的原始内容
func getYou2beRawData(html string) (you2beData map[string]interface{}) {
	patternJS := `(?msU)ytplayer.config = (.*?);ytplayer.load`
	patternStr := `ytplayer.config = |;ytplayer.load`
	regJS := regexp.MustCompile(patternJS)
	regStr := regexp.MustCompile(patternStr)
	y2bJS := regJS.FindAllString(html, -1)
	y2bJSON := regStr.ReplaceAllString(y2bJS[0], "")
	json.Unmarshal([]byte(y2bJSON), &you2beData)
	return
}

// findYou2beHighVideo 提取音视频最高码率
func findYou2beHighVideo(data []interface{}) (avMax map[string]int) {
	var videoBit []int
	var audioBit []int
	avMax = make(map[string]int)
	for _, videoInfo := range data {
		avgBit := videoInfo.(map[string]interface{})
		checkVideo := strings.Index(avgBit["mimeType"].(string), "video/mp4")
		checkAudio := strings.Index(avgBit["mimeType"].(string), "audio/mp4")
		if checkVideo != -1 {
			videoBit = append(videoBit, int(avgBit["averageBitrate"].(float64)))
		}
		if _, ok := avgBit["audioQuality"].(string); ok {
			if checkAudio != -1 {
				audioBit = append(audioBit, int(avgBit["averageBitrate"].(float64)))
			}
		}
	}
	avMax["maxVideo"] = maxInt(videoBit)
	avMax["maxAudio"] = maxInt(audioBit)
	return avMax
}

// md5key 字符串转md5
func md5key(str string) (md5str string) {
	w := md5.New()
	io.WriteString(w, str)
	md5str = fmt.Sprintf("%x", w.Sum(nil))
	return
}

// redisClient 连接 redis
func redisClient(addr string, pass string, db int) (client *redis.Client) {
	client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       db,
	})
	return
}

// checkLength 检查视频长度
func checkLength(data []interface{}) (length int) {
	var videoLength []int
	for _, videoInfo := range data {
		contentLength := videoInfo.(map[string]interface{})["contentLength"].(string)
		videoLen, _ := strconv.ParseInt(contentLength, 10, 64)
		videoLength = append(videoLength, int(videoLen))
	}
	length = maxInt(videoLength)
	return
}

// getYou2beVideoData 获取 youtube 最高质量音视频 URL
func getYou2beVideoData(raw map[string]interface{}) *VideoData {
	videoData := new(VideoData)
	tagAdaptiveFormatsData := make(map[string]interface{})
	tagArgs := raw["args"].(map[string]interface{})
	videoData.title = tagArgs["title"].(string)
	tagAdaptiveFormats := tagArgs["player_response"]
	json.Unmarshal([]byte(tagAdaptiveFormats.(string)), &tagAdaptiveFormatsData)
	streamingData := tagAdaptiveFormatsData["streamingData"].(map[string]interface{})
	adaptiveFormatsData := streamingData["adaptiveFormats"].([]interface{})
	avMax := findYou2beHighVideo(adaptiveFormatsData)
	maxVideoBit := avMax["maxVideo"]
	maxAudioBit := avMax["maxAudio"]
	maxLength := checkLength(adaptiveFormatsData)
	videoData.length = maxLength
	for _, data := range adaptiveFormatsData {
		maxVideo := data.(map[string]interface{})
		if int(maxVideo["averageBitrate"].(float64)) == maxVideoBit {
			videoData.videoURL = maxVideo["url"].(string)
		}
		if _, ok := maxVideo["audioQuality"].(string); ok {
			if int(maxVideo["averageBitrate"].(float64)) == maxAudioBit {
				videoData.audioURL = maxVideo["url"].(string)
			}
		}
	}
	return videoData
}

// request net/http 简单封装
func request(method string, durl string, r string, proxy bool) (*http.Response, error) {
	var client http.Client
	if proxy == true {
		proxyURL, _ := url.Parse("http://127.0.0.1:1080")
		tr := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		client = http.Client{Transport: tr, Timeout: 300 * time.Second}
	} else {
		client = http.Client{Timeout: 300 * time.Second}
	}
	req, _ := http.NewRequest(method, durl, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36")
	if r != "" {
		req.Header.Add("Range", "bytes="+r)
	}
	return client.Do(req)
}

// dlcoreMultiple 下载主函数
func dlcoreMultiple(vid int, url string, ranPart string, mainTmp string, dl *DLserver) {
	res, err := request("GET", url, ranPart, false)
	q := strconv.Itoa(vid)
	ranStart := strings.Split(ranPart, "-")[0]
	log.Println("Part " + q + " Downloading...")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	videoTmp, err := ioutil.TempFile(mainTmp, "youtube_"+q+"_"+ranStart+"_")
	if err != nil {
		panic(err)
	}
	defer videoTmp.Close()
	_, err = io.Copy(videoTmp, res.Body)
	if err != nil {
		panic(err)
	}
	dl.WG.Done()
	log.Println("Part " + q + " Finished ")
	<-dl.Gonum
}

// you2beDownload 下载器
func you2beDownload(uniqueName string, url string) (outputFilePath string, tmpPath string) {
	tmpPath, err := ioutil.TempDir("", "youtubeTmp")
	if err != nil {
		log.Fatal("Temporary file creation failed")
		return
	}
	dl := new(DLserver)
	res, _ := request("HEAD", url, "", false)
	resLength := res.ContentLength
	ranPart := rangePart(int(resLength), 8)
	tasks := len(ranPart)
	dl.WG.Add(tasks)
	dl.Gonum = make(chan string, 8)
	for vid, rPart := range ranPart {
		dl.Gonum <- rPart
		go dlcoreMultiple(vid+1, url, rPart, tmpPath, dl)
	}
	dl.WG.Wait()
	log.Println("Merge All Part...")
	outputFilePath = mergeFile(tmpPath, uniqueName)
	return
}

// mergeFile 合并分片文件
func mergeFile(tmpPath string, uniqueName string) string {
	mergeTmpPath := []string{tmpPath, uniqueName}
	mergeTmpFile := pathJoin(mergeTmpPath)
	saveFile, _ := os.Create(mergeTmpFile)
	tmpFiles, _ := ioutil.ReadDir(tmpPath)
	for _, f := range tmpFiles {
		if strings.Contains(f.Name(), ".") == false {
			seek, _ := strconv.ParseInt(strings.Split(f.Name(), "_")[2], 10, 64)
			fPath := []string{tmpPath, f.Name()}
			filename := pathJoin(fPath)
			rfile, _ := ioutil.ReadFile(filename)
			saveFile.WriteAt(rfile, seek)
			os.Remove(filename)
		}
	}
	return mergeTmpFile
}

func mergeMedia(video string, audio string, outname string) {
	// ffmpeg -v quiet -i {video} -i {audio} -map 0:0 -map 1:0 -vcodec copy -acodec copy {out}
	cmd := exec.Command("ffmpeg", "-y", "-v", "quiet", "-i", video, "-i", audio, "-map", "0:0", "-map", "1:0", "-c", "copy", outname)
	_, err := cmd.Output()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
}

// You2BeCoreData 视频信息
func You2BeCoreData(url string) (videodata *VideoData) {
	// 获取音视频URL
	log.Println("Get Video Info...")
	res, _ := request("GET", url, "", false)
	html, _ := ioutil.ReadAll(res.Body)
	youRawData := getYou2beRawData(string(html))
	videodata = getYou2beVideoData(youRawData)
	return
}

// You2BeGetVideo 下载视频
func You2BeGetVideo(savePath string, videodata *VideoData) (saveName string, videoTitle string) {
	uniqueName := time.Now().Format("20060102_150405")
	// 下载音视频
	videoURL := videodata.videoURL
	audioURL := videodata.audioURL
	videoTitle = videodata.title
	videoPath, tmpVPath := you2beDownload(uniqueName+".mp4", videoURL)
	audioPath, tmpAPath := you2beDownload(uniqueName+".m4a", audioURL)

	saveName = "y2b_" + uniqueName + ".mp4"
	savePathArray := []string{savePath, saveName}
	saveFilePath := pathJoin(savePathArray)
	// 合并音视频
	mergeMedia(videoPath, audioPath, saveFilePath)
	os.RemoveAll(tmpVPath)
	os.RemoveAll(tmpAPath)
	return
}

func main() {
	var mainCfg *ConfigInfo
    mainCfg = getConfig()
    gin.SetMode(gin.ReleaseMode)
    log.Println("Listening and serving HTTP on " + mainCfg.listenAddr)
	client := redisClient(mainCfg.redisAddr, "", 0)
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"POST", "OPTION"}
	router := gin.Default()
    router.Use(cors.New(config))
	router.POST("/api/y2b", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		var reqData URLData
		var y2bData *VideoData
		if c.BindJSON(&reqData) == nil {
			time.Sleep(2 * time.Second)
			var durl string
			var videoTitle string
			var saveName string
			vurl := reqData.URL

			y2bData = You2BeCoreData(vurl)
			length := y2bData.length
			if length > 209715200 {
				c.JSON(http.StatusOK, gin.H{
					"status":  1,
					"message": "Video size limit ( <200M )",
				})
			} else {
				md5URL := md5key(vurl)
				rdisMedia, err := client.Get(md5URL).Result()
				if err != nil {
					savePath := mainCfg.savePath
					wwwPath := mainCfg.wwwPath
					saveName, videoTitle = You2BeGetVideo(savePath, y2bData)
					wwwPathJ := []string{wwwPath, saveName}
					durl = pathJoin(wwwPathJ)
					client.Set(md5URL, durl, 0)
				} else {
					durl = rdisMedia
					videoTitle = y2bData.title
				}
				c.JSON(http.StatusOK, gin.H{
					"status":       0,
					"download_url": durl,
					"title":        videoTitle,
					"url":          durl,
				})
			}
		} else {
			c.JSON(http.StatusOK, gin.H{
				"status":  1,
				"message": "para error",
			})
		}
	})
	router.Run(mainCfg.listenAddr)
}
