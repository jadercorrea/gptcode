package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jadercorrea/gptcode/internal/live"
)

func main() {
	fmt.Println("🔗 GT Live Dashboard Test")
	fmt.Println("==========================")

	// Note: model configuration is handled by the Live client internally

	// Create client
	url := "https://gptcode.live"
	agentID := live.GetAgentIDWithType("builder")
	fmt.Printf("Agent ID: %s\n", agentID)

	client := live.NewClient(url, agentID)
	client.SetAgentType("builder")
	client.SetTask("Testing GT live reporting")

	// Connect
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	fmt.Println("✅ Connected to Live Dashboard")

	// Report config for HTTP fallback
	reportConfig := live.DefaultReportConfig()
	reportConfig.SetBaseURL(url)
	if err := reportConfig.Connect(agentID, "builder", "Testing live reporting"); err != nil {
		fmt.Printf("⚠️ HTTP connect failed: %v\n", err)
	} else {
		fmt.Println("✅ HTTP reported to Live")
	}

	// Set callbacks for receiving messages
	client.OnCommand(func(cmd string, payload map[string]interface{}) {
		fmt.Printf("\n📨 Command received: %s\n", cmd)
		fmt.Printf("   Payload: %v\n", payload)
	})

	client.OnContextEdit(func(ctxType, content string) {
		fmt.Printf("\n📝 Context edit: [%s]\n%s\n", ctxType, content)
	})

	// Report steps
	reportConfig.Step("Connected and ready", "start")
	client.SendExecutionStep("status", "Connected and waiting for messages", nil)

	// Send periodic updates
	go func() {
		for i := 1; i <= 5; i++ {
			time.Sleep(3 * time.Second)
			msg := fmt.Sprintf("Heartbeat #%d - all systems operational", i)
			client.SendExecutionStep("heartbeat", msg, nil)
			reportConfig.Step(msg, "heartbeat")
			fmt.Printf("📤 Sent: %s\n", msg)
		}
	}()

	// Wait for messages
	fmt.Println("\n👂 Listening for messages from Live Dashboard...")
	fmt.Println("   (Press Ctrl+C to exit)")

	// Keep running
	done := make(chan bool)
	<-done
}
