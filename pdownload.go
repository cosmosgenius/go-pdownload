package pdownload

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

//Download download file from <urlStr> to <filepath> use <concurrency> goroutines
func Download(urlStr string, filepath string, concurrency int) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	resp, err := http.Get(urlStr)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if hasAcceptRanges(resp) {
		contentLength := resp.ContentLength
		return startConcurrentDownload(filepath, parsedURL, contentLength, concurrency)
	}

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	requestToFile(req, filepath)

	return err
}

func hasAcceptRanges(resp *http.Response) bool {
	acceptRanges := resp.Header["Accept-Ranges"]
	for _, value := range acceptRanges {
		lowerValue := strings.ToLower(value)
		if lowerValue == "bytes" {
			return true
		}
	}
	return false
}

func startConcurrentDownload(filepath string, downloadurl *url.URL, contentLength int64, concurrency int) error {
	chunkSize := contentLength / int64(concurrency)
	prefix := filepath + "part"

	var wg sync.WaitGroup
	wg.Add(concurrency)

	var lastbit int64
	var endbit int64
	lastbit = 0

	for i := 0; i < concurrency; i++ {
		endbit = lastbit + chunkSize
		go downloadPart(downloadurl, lastbit, endbit, prefix, i, &wg)
		lastbit = endbit + 1
	}

	wg.Wait()
	os.Remove(filepath)
	err := mergeFiles(prefix, concurrency, filepath)

	if err != nil {
		return err
	}

	for i := 0; i < concurrency; i++ {
		prefixfilename := prefix + "." + strconv.Itoa(i)
		os.Remove(prefixfilename)
	}
	return nil
}

func mergeFiles(prefix string, parts int, outfile string) error {
	out, err := os.Create(outfile)
	if err != nil {
		return err
	}
	for i := 0; i < parts; i++ {
		filename := prefix + "." + strconv.Itoa(i)
		_, err := mergeFileInto(out, filename)
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeFileInto(outfile *os.File, infile string) (*int64, error) {
	in, err := os.Open(infile)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	num, err := io.Copy(outfile, in)
	if err != nil {
		return nil, err
	}
	return &num, nil
}

func downloadPart(
	durl *url.URL, startbit int64, endbit int64,
	prefix string, part int, wg *sync.WaitGroup,
) {
	defer wg.Done()
	filename := prefix + "." + strconv.Itoa(part)
	req, _ := http.NewRequest("GET", durl.String(), nil)
	rangeHeader := fmt.Sprintf("bytes=%d-%d", startbit, endbit)
	req.Header.Add("Range", rangeHeader)
	requestToFile(req, filename)
}

func requestToFile(req *http.Request, filename string) error {
	os.Remove(filename)
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()
	client := http.Client{}
	attempts := 4
	var responseErr error
	responseErr = nil
	for attempts > 0 {
		attempts--
		resp, err := client.Do(req)
		responseErr = err
		if err != nil {
			continue
		}
		io.Copy(out, resp.Body)
		resp.Body.Close()
		return nil
	}
	return responseErr
}
