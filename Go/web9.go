package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/mux"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type Response struct {
	ContainerID string `json:"container_id"`
	IPAddress   string `json:"ip_address"`
}

func createDockerContainer(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ram, err := strconv.ParseInt(r.FormValue("ram"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid input for RAM", http.StatusBadRequest)
		return
	}

	cpu, err := strconv.ParseInt(r.FormValue("cpu"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid input for CPU", http.StatusBadRequest)
		return
	}

	hostPortStr := r.FormValue("hostPort")
	hostPortInt, err := strconv.Atoi(hostPortStr)
	if err != nil {
		http.Error(w, "Invalid input for host port", http.StatusBadRequest)
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		http.Error(w, "Failed to list containers", http.StatusInternalServerError)
		return
	}
	for _, container := range containers {
		for _, port := range container.Ports {
			if port.PublicPort == uint16(hostPortInt) {
				errorResponse := ErrorResponse{
					Message: "Port is already allocated",
				}
				responseJSON, err := json.Marshal(errorResponse)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, err = w.Write(responseJSON)
				if err != nil {
					log.Println("Failed to write response:", err)
				}
				return
			}
		}
	}

	config := &container.Config{
		Image: "ombansod", 
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:     ram,
			MemorySwap: ram,
			NanoCPUs:   cpu * 1e9,
		},
		PortBindings: nat.PortMap{
			"80/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPortStr,
				},
			},
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	containerIP, err := getContainerIPAddress(resp.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := Response{
		ContainerID: resp.ID,
		IPAddress:   containerIP,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}

func getContainerResources(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	containerID := params["id"]

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type Response struct {
		CPU int64 `json:"cpu"`
		RAM int64 `json:"ram"`
	}

	response := Response{
		CPU: containerInfo.HostConfig.Resources.NanoCPUs / 1e9,
		RAM: containerInfo.HostConfig.Resources.Memory,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}

func editContainerResources(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	containerID := params["id"]

	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ram, err := strconv.ParseInt(r.FormValue("ram"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid input for RAM", http.StatusBadRequest)
		return
	}

	cpu, err := strconv.ParseInt(r.FormValue("cpu"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid input for CPU", http.StatusBadRequest)
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	resources := container.UpdateConfig{
		Resources: container.Resources{
			Memory:     ram,
			MemorySwap: ram,
			NanoCPUs:   cpu * 1e9,
		},
	}

	_, err = cli.ContainerUpdate(ctx, containerID, resources)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type Response struct {
		CPU int64 `json:"cpu"`
		RAM int64 `json:"ram"`
	}

	response := Response{
		CPU: containerInfo.HostConfig.Resources.NanoCPUs / 1e9,
		RAM: containerInfo.HostConfig.Resources.Memory,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}

func getContainerIPAddress(containerID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	if len(containerInfo.NetworkSettings.Networks) == 0 {
		return "", nil
	}

	var containerIP string
	for _, network := range containerInfo.NetworkSettings.Networks {
		containerIP = network.IPAddress
		break
	}

	return containerIP, nil
}

func listContainers(w http.ResponseWriter, r *http.Request) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		http.Error(w, "Failed to list containers", http.StatusInternalServerError)
		return
	}

	var containerInfo []struct {
		ID    string   `json:"ID"`
		Names []string `json:"Names"`
	}

	for _, container := range containers {
		containerInfo = append(containerInfo, struct {
			ID    string   `json:"ID"`
			Names []string `json:"Names"`
		}{
			ID:    container.ID,
			Names: container.Names,
		})
	}

	responseJSON, err := json.Marshal(containerInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}

func stopContainer(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	containerID := r.FormValue("containerID")
	log.Println("Received containerID:", containerID)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	stopOptions := container.StopOptions{}

	log.Println("Attempting to stop container with ID:", containerID)

	if err := cli.ContainerStop(ctx, containerID, stopOptions); err != nil {
		log.Println("Error stopping container:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Message string `json:"message"`
	}{
		Message: "Container stopped successfully",
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}

func main() {
	router := mux.NewRouter()

	// Serve HTML page and static assets
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	router.HandleFunc("/containers", createDockerContainer).Methods("POST")
	router.HandleFunc("/containers/{id}/resources", getContainerResources).Methods("GET")
	router.HandleFunc("/containers/{id}/resources", editContainerResources).Methods("PUT")
	router.HandleFunc("/containers/stop", stopContainer).Methods("POST")
	router.HandleFunc("/containers", listContainers).Methods("GET") // New route to list containers

	log.Println("Server started on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
