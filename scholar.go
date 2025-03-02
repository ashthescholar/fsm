package main
import (
	"fmt"
	"os"
	"strings"
    "encoding/json"
	"bytes"
	"io/ioutil"
	"net/http"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)



// Replace with your actual Gemini API Key
const apiKey = "APIKEYHERE"

var (
	terminalWidth    = 70
	orangeRedColor   = lipgloss.Color("#CF7C71")
	grayText         = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	commandListStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("245")).
				Padding(1, 2).
				Width(50)

	banner = lipgloss.NewStyle().Foreground(orangeRedColor).
		Render(`
	   
             █████╗ ███████╗██╗  ██╗ ██████╗██╗      █████╗ ██╗   ██╗██████╗ ███████╗
			██╔══██╗██╔════╝██║  ██║██╔════╝██║     ██╔══██╗██║   ██║██╔══██╗██╔════╝
			███████║███████╗███████║██║     ██║     ███████║██║   ██║██║  ██║█████╗  
			██╔══██║╚════██║██╔══██║██║     ██║     ██╔══██║██║   ██║██║  ██║██╔══╝  
			██║  ██║███████║██║  ██║╚██████╗███████╗██║  ██║╚██████╔╝██████╔╝███████╗
			╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝
		`)

	welcomeBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(orangeRedColor).
			Foreground(lipgloss.Color("15")).
			Padding(1).
			Width(50).
			Align(lipgloss.Center).
			Bold(false).
			Render("✱ Welcome to " + lipgloss.NewStyle().Bold(true).Render("ashclaude."))

	inputBox = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#000000")).
			Width(70).
			Padding(0, 1)

	textBelowInput = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Render("! for bash mode  ·  / for commands  ·  tab to undo  ·  ↵ for newline")

	commands = []string{
		"/clear - Clear conversation history",
		"/compact - Clear history but keep summary",
		"/config - Open config panel",
		"/cost - Show total cost and duration",
		"/doctor - Check system health",
		"/help - Show help",
		"/init - Initialize CLAUDE.md file",
		"/pr-comments - Get comments from GitHub PR",
		"/bug - Submit feedback",
		"/review - Review a PR",
		"/terminal-setup - Install Shift+Enter binding",
		"/logout - Sign out",
		"/login - Switch account",
	}
	processingStyle = lipgloss.NewStyle().Foreground(orangeRedColor)
)
type model struct {
	spinner       spinner.Model
	processing    bool
	messages      []string
	input         string
	showCommands  bool
	selectedIndex int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			m.showCommands = true
			m.selectedIndex = 0
			return m, nil

		case "%":
			if m.showCommands && m.selectedIndex < len(commands)-1 {
				m.selectedIndex++
			}
			return m, nil

		case "@":
			if m.showCommands && m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil

		case "*":
			m.showCommands = false
			return m, tea.Quit

		case "enter":
			if m.showCommands {
				m.input = strings.Split(commands[m.selectedIndex], " - ")[0] + " "
				m.showCommands = false
			} else {
				if strings.TrimSpace(m.input) == "/esc" {
					fmt.Println("Exiting...")
					return m, tea.Quit
				}
				userMessage := grayText.Render("> " + m.input)
				m.messages = append(m.messages, userMessage)
				userInput := m.input
				m.input = ""
				m.processing = true

				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					response, err := fetchGeminiResponse(m, userInput)
					if err != nil {
						return geminiResponseMsg{response: "Error: " + err.Error()}
					}
					return geminiResponseMsg{response: response}
				})
			}
			return m, nil

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil

		default:
			if !m.showCommands {
				m.input += msg.String()
			}
		}
	case spinner.TickMsg:
		if m.processing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case geminiResponseMsg:
		m.processing = false
		wrappedResponse := wrapText(msg.response, terminalWidth)
		m.messages = append(m.messages, processingStyle.Render("> " + wrappedResponse))
	}
	return m, nil
}
func (m model) View() string {
	var s string
	s += banner + "\n\n" + welcomeBox + "\n\n"
	for _, msg := range m.messages {
		s += msg + "\n"
	}
	if m.processing {
		s += processingStyle.Render("Processing " + m.spinner.View()) + "\n\n"
	} else if m.showCommands {
		s += renderCommandMenu(m.selectedIndex) + "\n\n"
	}
	s += inputBox.Render("> " + m.input) + "\n"
	s += textBelowInput + "\n"

	return s
}

// Struct to encapsulate Gemini responses
type geminiResponseMsg struct {
	response string
}
func renderCommandMenu(selected int) string {
	var renderedCommands string
	for i, cmd := range commands {
		if i == selected {
			renderedCommands += lipgloss.NewStyle().Foreground(orangeRedColor).Bold(true).Render("> " + cmd) + "\n"
		} else {
			renderedCommands += grayText.Render("  " + cmd) + "\n"
		}
	}
	return commandListStyle.Render(renderedCommands)
}
// Function to send conversation history to Gemini
func fetchGeminiResponse(m model, userInput string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", apiKey)

	// Build conversation history
	var conversationHistory []map[string]string
	// add "text" to each entry
	for _, msg := range m.messages {
		conversationHistory = append(conversationHistory, map[string]string{"text": msg})
	}
	//append entry
	conversationHistory = append(conversationHistory, map[string]string{"text": userInput})

	// Create request body
	requestBody, err := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": conversationHistory},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode request: %v", err)
	}

	// Send HTTP request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Extract and return the text response
	if candidates, ok := result["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if firstCandidate, ok := candidates[0].(map[string]interface{}); ok {
			if content, ok := firstCandidate["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
					if textMap, ok := parts[0].(map[string]interface{}); ok {
						if text, ok := textMap["text"].(string); ok {
							return text, nil
						}
					}
				}
			}
		}
	}

	return "No response from Gemini.", nil
}


// Wrap text to fit terminal width
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var wrappedText, line string
	for _, word := range words {
		if len(line)+len(word)+1 > width {
			wrappedText += line + "\n"
			line = word
		} else {
			if line != "" {
				line += " "
			} line += word}}
	if line != "" {
		wrappedText += line
	}
	return wrappedText
}

// Main function
func main() {
	s := spinner.New()
	s.Spinner = spinner.Jump
	s.Style = lipgloss.NewStyle().Foreground(orangeRedColor)
	p := tea.NewProgram(model{spinner: s})

	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
