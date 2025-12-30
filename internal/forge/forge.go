package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type ForgeClient struct {
	apiKey  string
	baseURL string
}

type ForgeServer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip_address"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type ForgeSite struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ServerID string `json:"server_id"`
	Status   string `json:"status"`
}

func NewForgeClient(apiKey string) *ForgeClient {
	return &ForgeClient{
		apiKey:  apiKey,
		baseURL: "https://forge.laravel.com/api/v1",
	}
}

func (fc *ForgeClient) GetServers() ([]ForgeServer, error) {
	req, _ := http.NewRequest("GET", fc.baseURL+"/servers", nil)
	req.Header.Set("Authorization", "Bearer "+fc.apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Servers []ForgeServer `json:"servers"`
	}

	json.NewDecoder(resp.Body).Decode(&result)
	return result.Servers, nil
}

func (fc *ForgeClient) GetSites(serverID string) ([]ForgeSite, error) {
	req, _ := http.NewRequest("GET", fc.baseURL+"/servers/"+serverID+"/sites", nil)
	req.Header.Set("Authorization", "Bearer "+fc.apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Sites []ForgeSite `json:"sites"`
	}

	json.NewDecoder(resp.Body).Decode(&result)
	return result.Sites, nil
}

func (fc *ForgeClient) DeploySite(serverID, siteID string) error {
	req, _ := http.NewRequest("POST", fc.baseURL+"/servers/"+serverID+"/sites/"+siteID+"/deploy", nil)
	req.Header.Set("Authorization", "Bearer "+fc.apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deployment failed with status %d", resp.StatusCode)
	}

	return nil
}

func (fc *ForgeClient) GetSiteEnv(serverID, siteID string) (string, error) {
	req, _ := http.NewRequest("GET", fc.baseURL+"/servers/"+serverID+"/sites/"+siteID+"/env", nil)
	req.Header.Set("Authorization", "Bearer "+fc.apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Content string `json:"content"`
	}

	json.NewDecoder(resp.Body).Decode(&result)
	return result.Content, nil
}

func (fc *ForgeClient) UpdateSiteEnv(serverID, siteID, content string) error {
	body, _ := json.Marshal(map[string]string{
		"content": content,
	})

	req, _ := http.NewRequest("PUT", fc.baseURL+"/servers/"+serverID+"/sites/"+siteID+"/env", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+fc.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update failed with status %d", resp.StatusCode)
	}

	return nil
}
