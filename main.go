package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

var (
	tekton_latest_install_url  = "https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml"
	gcr_io_tekton_prefix       = "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/"
	dockerhub_image_ns         = "docker.io/shenchangqing/"
	cgr_dev_chainguard_busybox = "cgr.dev/chainguard/"
	mcr_microsoft_powershell   = "mcr.microsoft.com/"
	dockerhub_image_prefix     = "tekton-"
)

func main() {

	var gen_bash_script, gen_install_yaml bool

	flag.BoolVar(&gen_bash_script, "script", true, "generate build and push bash script")
	flag.BoolVar(&gen_install_yaml, "yaml", false, "generate install yaml")
	flag.Parse()

	gcrImages, tekton_install_yaml_byte := GetGCRImages(tekton_latest_install_url)
	dockerImages := GetDockerHubImages(gcrImages)

	if gen_install_yaml {
		newYaml := GenerateDockerHubInstallYaml(gcrImages, dockerImages, tekton_install_yaml_byte)
		fmt.Println(newYaml)
		return
	}
	if gen_bash_script {
		res := GenerateBuildAndPushShell(gcrImages, dockerImages)
		fmt.Println(res)
	}

}

func GenerateDockerHubInstallYaml(gcrImages, dockerImages []string, ori_install_yaml_byte []byte) string {
	var docker_install_yaml string

	if len(gcrImages) == 0 || len(gcrImages) != len(dockerImages) {
		slog.Error("gcrImages为空或与dockerImages不相等，请检查")
		return docker_install_yaml
	}

	docker_install_yaml = string(ori_install_yaml_byte)
	fmt.Println(docker_install_yaml)

	for i := range gcrImages {
		docker_install_yaml = strings.ReplaceAll(docker_install_yaml, gcrImages[i], dockerImages[i])
	}

	return docker_install_yaml

}

func GenerateBuildAndPushShell(gcrImgs, dockerImgs []string) string {

	if len(gcrImgs) == 0 || len(gcrImgs) != len(dockerImgs) {
		slog.Info("gcrImages为空，请检查")
		return ""
	}

	var builder strings.Builder
	builder.WriteString("#!/bin/bash\n")

	// pull
	for i := range dockerImgs {
		builder.WriteString("docker pull" + " " + gcrImgs[i] + "\n")
	}

	// tag
	for i := range dockerImgs {
		builder.WriteString("docker tag" + " " + gcrImgs[i] + " " + dockerImgs[i] + "\n")
	}

	// push
	for i := range dockerImgs {
		builder.WriteString("docker push" + " " + dockerImgs[i] + "\n")
	}

	return builder.String()

}

func GetDockerHubImages(gcrImages []string) []string {
	docker_images_all := []string{}

	for _, v := range gcrImages {
		gcr_image_no_hash, _, _ := strings.Cut(v, "@sha256")
		gcr_image_no_hash = strings.ReplaceAll(gcr_image_no_hash, gcr_io_tekton_prefix, dockerhub_image_ns+dockerhub_image_prefix)
		gcr_image_no_hash = strings.ReplaceAll(gcr_image_no_hash, cgr_dev_chainguard_busybox, dockerhub_image_ns+dockerhub_image_prefix)
		gcr_image_no_hash = strings.ReplaceAll(gcr_image_no_hash, mcr_microsoft_powershell, dockerhub_image_ns+dockerhub_image_prefix)
		docker_images_all = append(docker_images_all, gcr_image_no_hash)
	}

	return docker_images_all
}

func GetGCRImages(tektonLatestUrl string) ([]string, []byte) {

	gcr_images_all := []string{}
	install_yaml_byte := []byte{}

	res, err := http.Get(tektonLatestUrl)
	if err != nil || res.StatusCode != 200 {
		slog.Error("http.Get()", "url", tektonLatestUrl, "msg", err)
		return gcr_images_all, install_yaml_byte
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("io.ReadAll res.body", "msg", err)
		return gcr_images_all, install_yaml_byte
	}
	rb := io.NopCloser(bytes.NewReader(b))
	defer rb.Close()

	// f, err := os.Open(fileName)
	// if err != nil {
	// 	slog.Error("file not found, please check", "name", fileName)
	// 	return gcr_images_all
	// }
	// defer f.Close()
	scanner := bufio.NewScanner(rb)
	trim_f := func(r rune) bool {
		return string(r) == `"` || string(r) == `[` || string(r) == `]`
	}
	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, gcr_io_tekton_prefix) ||
			strings.Contains(line, cgr_dev_chainguard_busybox) ||
			strings.Contains(line, mcr_microsoft_powershell) {

			tmpStr := strings.TrimPrefix(line, "image: ")
			tmpStr2 := strings.ReplaceAll(tmpStr, " ", "")
			tmpStr3 := strings.Split(tmpStr2, ",")

			for _, tmp := range tmpStr3 {
				if strings.Contains(tmp, gcr_io_tekton_prefix) ||
					strings.Contains(tmp, cgr_dev_chainguard_busybox) ||
					strings.Contains(tmp, mcr_microsoft_powershell) {
					gcr_images_all = append(gcr_images_all, strings.TrimFunc(tmp, trim_f))
				}

			}

		}
	}

	return gcr_images_all, b

}
