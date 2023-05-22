package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/liudy1993/injector/proto"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

var (
	workflowsJson  []WorkflowsJson
	workflowsProto []WorkflowsProto
)

type WorkflowsJson struct {
	Json []byte
}
type WorkflowsProto struct {
	Proto []byte
}

func readDag() {
	var jsonFilesPath []string
	var protoFilesPath []string

	file, err := os.Open("files.txt")
	if err != nil {
		log.Println("无法打开文件：", err)
		return
	}
	defer file.Close()

	// 逐行读取文本文件
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// 检查行内容是否为JSON或Proto文件地址
		if strings.HasPrefix(line, "json:") {
			jsonFilesPath = append(jsonFilesPath, line[5:])
		} else if strings.HasPrefix(line, "protobuf:") {
			protoFilesPath = append(protoFilesPath, line[9:])
		}
	}

	// 读取JSON文件
	for _, path := range jsonFilesPath {
		jsonFile, err := os.ReadFile(path)
		if err != nil {
			log.Println("无法打开文件：", err)
			return
		}
		workflowsJson = append(workflowsJson, WorkflowsJson{Json: jsonFile})
	}

	// 读取Proto文件
	for _, path := range protoFilesPath {
		protoFile, err := os.ReadFile(path)
		if err != nil {
			log.Println("无法打开文件：", err)
			return
		}
		workflowsProto = append(workflowsProto, WorkflowsProto{Proto: protoFile})
	}
}

func jsonToWorkflowYaml(workflowJson []byte) []byte {
	//定义结构
	type Task struct {
		Name         string   `json:"name"`
		Dependencies []string `json:"dependencies"`
		Template     string   `json:"template"`
	}
	type Workflow struct {
		WorkflowName string `json:"workflow_name"`
		Style        string `json:"style"`
		CustomID     string `json:"custom_id"`
		Topology     []Task `json:"topology"`
	}
	type Template struct {
		Name      string `yaml:"name"`
		Container struct {
			Image           string `yaml:"image"`
			ImagePullPolicy string `yaml:"imagePullPolicy"`
			Resources       struct {
				Limits struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"limits"`
				Requests struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"requests"`
			} `yaml:"resources"`
		} `yaml:"container,omitempty"`
		Dag struct {
			Tasks []Task `yaml:"tasks"`
		} `yaml:"dag,omitempty"`
	}
	type YamlWorkflow struct {
		ApiVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			GenerateName string `yaml:"generateName"`
		} `yaml:"metadata"`
		Spec struct {
			Entrypoint string `yaml:"entrypoint"`
			PodGC      struct {
				Strategy string `yaml:"strategy"`
			} `yaml:"podGC"`
			TtlStrategy struct {
				SecondsAfterCompletion int `yaml:"secondsAfterCompletion"`
			} `yaml:"ttlStrategy"`
			Template []Template `yaml:"templates"`
		} `yaml:"spec"`
	}

	//解析json文件
	var workflow Workflow
	if err := json.Unmarshal(workflowJson, &workflow); err != nil {
		log.Println(err)
	}
	//解析json文件中的topology
	var tasks []Task
	for _, typl := range workflow.Topology {
		var task Task
		task.Name = typl.Name
		task.Dependencies = typl.Dependencies
		task.Template = "task"
		tasks = append(tasks, task)
	}
	//生成yaml文件
	var yamlWorkflow YamlWorkflow
	yamlWorkflow.ApiVersion = "argoproj.io/v1alpha1"
	yamlWorkflow.Kind = "Workflow"
	yamlWorkflow.Metadata.GenerateName = "argo-test-wf-"
	yamlWorkflow.Spec.Entrypoint = workflow.WorkflowName
	yamlWorkflow.Spec.PodGC.Strategy = "OnPodSuccess"
	yamlWorkflow.Spec.TtlStrategy.SecondsAfterCompletion = 60
	yamlWorkflow.Spec.Template = []Template{
		{
			Name: "task",
			Container: struct {
				Image           string `yaml:"image"`
				ImagePullPolicy string `yaml:"imagePullPolicy"`
				Resources       struct {
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
				} `yaml:"resources"`
			}{
				Image:           "harbor.cloudcontrolsystems.cn/workflow/task:latest",
				ImagePullPolicy: "IfNotPresent",
				Resources: struct {
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
				}{
					Limits: struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					}{
						CPU:    "2000m",
						Memory: "128Mi",
					},
					Requests: struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					}{
						CPU:    "1000m",
						Memory: "64Mi",
					},
				},
			},
		},
		{
			Name: "NoName",
			Dag: struct {
				Tasks []Task `yaml:"tasks"`
			}{
				Tasks: tasks,
			},
		},
	}
	//将yaml文件转换为字节流
	yamlBytes, err := yaml.Marshal(&yamlWorkflow)
	if err != nil {
		log.Println(err)
	}
	return yamlBytes
}

func sendToArgo(workflowJson []byte) int64 {
	a1 := time.Now().UnixNano()
	yaml := jsonToWorkflowYaml(workflowJson)
	//写入linux文件
	err := os.WriteFile("123.yaml", yaml, 0644)
	if err != nil {
		log.Println(err)
	}
	//执行argo提交工作流命令
	cmd := exec.Command("/bin/argo", "submit", "-n", "argo", "123.yaml")
	err = cmd.Run()
	if err != nil {
		log.Println(err)
	}

	log.Println("Argo发送成功")

	return time.Now().Unix() - a1
}

func sendToSc(workflow []byte, NEW_CORE_ADDRESS string) int64 {
	a1 := time.Now().UnixNano()

	channle, err := grpc.Dial(NEW_CORE_ADDRESS, grpc.WithInsecure())
	if err != nil {
		log.Println(err)
	}
	controller := proto.NewSchedulerControllerClient(channle)

	req := &proto.InputWorkflowRequest{
		Workflow: [][]byte{workflow},
	}

	reply, err := controller.InputWorkflow(context.Background(), req)
	if reply != nil {
		log.Println("SC发送成功")
	} else {
		log.Println(err)
	}

	_ = channle.Close()

	return time.Now().UnixNano() - a1

}

func main() {
	log.Println("Strat!")
	os.Setenv("NEW_CORE_ADDRESS", "172.28.0.90:6060")
	os.Setenv("TEST_WITH_ARGO", "false")
	os.Setenv("TEST_WITH_SC", "true")
	os.Setenv("BATCH_SIZE", "10")
	os.Setenv("SC_TYPE", "snappy")

	NEW_CORE_ADDRESS := os.Getenv("NEW_CORE_ADDRESS")
	TEST_WITH_ARGO := os.Getenv("TEST_WITH_ARGO")
	TEST_WITH_SC := os.Getenv("TEST_WITH_SC")
	BATCH_SIZE := os.Getenv("BATCH_SIZE")
	SC_TYPE := os.Getenv("SC_TYPE")

	readDag()

	batchSize, err := strconv.Atoi(BATCH_SIZE)
	if err != nil {
		log.Println(err)
	}

	if TEST_WITH_ARGO == "true" {
		log.Println("Start to send workflow to argo!")
		for i, workflow := range workflowsJson {
			sendToArgo(workflow.Json)
			if i >= batchSize-1 {
				break
			}

		}
	}
	if TEST_WITH_SC == "true" {
		log.Println("Start to send workflow to scheduler controller!")

		if SC_TYPE == "snappy" {
			for i, workflow := range workflowsProto {
				sendToSc(workflow.Proto, NEW_CORE_ADDRESS)
				if i >= batchSize-1 {
					break
				}
			}
		}
		if SC_TYPE == "json" {
			for i, workflow := range workflowsJson {
				sendToSc(workflow.Json, NEW_CORE_ADDRESS)
				if i >= batchSize-1 {
					break
				}
			}
		}
	}

}
