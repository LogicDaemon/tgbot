package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Secrets holds only the authentication secrets for the bot
type Secrets struct {
	TelegramBotToken string `json:"telegram_bot_token"`
}

// Settings holds the configuration settings for the bot
type Settings struct {
	TelegramChannelID int64 `json:"telegram_channel_id"`
}

// LastPostData stores information about the last posted message
type LastPostData struct {
	LastPostURL string `json:"last_post_url"` // URL of the last posted news item
}

// NewsItem represents a news item
type NewsItem struct {
	Title          string
	URL            string
	Date           string
	Text           string
	SectionContent string
}

const (
	magticomNewsURL = "https://www.magticom.ge/en/about-company/news"
)

func getDefaultSecretsPath() string {
	var secretDataDir string

	// Check environment variable
	if envPath := os.Getenv("SECRETS_PATH"); envPath != "" {
		return envPath
	}

	// Check SecretDataDir environment variable
	if dir := os.Getenv("SecretDataDir"); dir != "" {
		secretDataDir = dir
	} else {
		// Default paths based on OS
		if runtime.GOOS == "windows" {
			localAppData := os.Getenv("LOCALAPPDATA")
			secretDataDir = filepath.Join(localAppData, "_sec")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("Error getting home directory: %v", err)
			}
			secretDataDir = filepath.Join(homeDir, ".local", "_sec")
		}
	}

	return filepath.Join(secretDataDir, "repost_magti_news.json")
}

func getLastPostDataPath() string {
	var dataDir string

	// Default paths based on OS
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		dataDir = filepath.Join(localAppData, "repost_magti_news")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting home directory: %v", err)
		}
		dataDir = filepath.Join(homeDir, ".local", "repost_magti_news")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	return filepath.Join(dataDir, "last_post.json")
}

func getSettingsPath() string {
	var dataDir string

	// Default paths based on OS
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		dataDir = filepath.Join(localAppData, "repost_magti_news")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting home directory: %v", err)
		}
		dataDir = filepath.Join(homeDir, ".local", "repost_magti_news")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	return filepath.Join(dataDir, "settings.json")
}

func loadSecrets() (*Secrets, error) {
	secretsPath := getDefaultSecretsPath()

	// Check if file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("secrets file not found at %s", secretsPath)
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading secrets file: %v", err)
	}

	var secrets Secrets
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("error parsing secrets file: %v", err)
	}

	if secrets.TelegramBotToken == "" {
		return nil, fmt.Errorf("missing required secrets")
	}

	return &secrets, nil
}

// func saveSecrets(secrets *Secrets) error {
// 	secretsPath := getDefaultSecretsPath()

// 	// Ensure directory exists
// 	dir := filepath.Dir(secretsPath)
// 	if err := os.MkdirAll(dir, 0700); err != nil {
// 		return fmt.Errorf("error creating directory: %v", err)
// 	}

// 	data, err := json.MarshalIndent(secrets, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("error marshaling secrets: %v", err)
// 	}

// 	if err := os.WriteFile(secretsPath, data, 0600); err != nil {
// 		return fmt.Errorf("error writing secrets file: %v", err)
// 	}

// 	return nil
// }

func loadSettings() (*Settings, error) {
	secretsPath := getSettingsPath()

	// Check if file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("settings file not found at %s", secretsPath)
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading settings file: %v", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("error parsing settings file: %v", err)
	}

	if settings.TelegramChannelID == 0 {
		return nil, fmt.Errorf("missing required settings")
	}

	return &settings, nil
}

// func saveSettings(settings *Settings) error {
// 	secretsPath := getSettingsPath()

// 	// Ensure directory exists
// 	dir := filepath.Dir(secretsPath)
// 	if err := os.MkdirAll(dir, 0700); err != nil {
// 		return fmt.Errorf("error creating directory: %v", err)
// 	}

// 	data, err := json.MarshalIndent(settings, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("error marshaling settings: %v", err)
// 	}

// 	if err := os.WriteFile(secretsPath, data, 0600); err != nil {
// 		return fmt.Errorf("error writing settings file: %v", err)
// 	}

// 	return nil
// }

func loadLastPostData() (*LastPostData, error) {
	lastPostPath := getLastPostDataPath()

	// If file doesn't exist, return an empty struct
	if _, err := os.Stat(lastPostPath); os.IsNotExist(err) {
		return &LastPostData{
			LastPostURL: "",
		}, nil
	}

	data, err := os.ReadFile(lastPostPath)
	if err != nil {
		return nil, fmt.Errorf("error reading last post data file: %v", err)
	}

	var lastPostData LastPostData
	if err := json.Unmarshal(data, &lastPostData); err != nil {
		return nil, fmt.Errorf("error parsing last post data file: %v", err)
	}

	return &lastPostData, nil
}

func saveLastPostData(lastPostData *LastPostData) error {
	lastPostPath := getLastPostDataPath()

	data, err := json.MarshalIndent(lastPostData, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling last post data: %v", err)
	}

	if err := os.WriteFile(lastPostPath, data, 0600); err != nil {
		return fmt.Errorf("error writing last post data file: %v", err)
	}

	return nil
}

func sendToTelegram(botToken string, channelID int64, newsItem NewsItem) error {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return fmt.Errorf("error initializing bot: %v", err)
	}

	// Format the date nicely
	dateText := strings.TrimSpace(newsItem.Text)

	// The section content is already formatted by parseHtmlContent
	content := newsItem.SectionContent

	// Format message with proper spacing
	message := fmt.Sprintf("📅 %s\n\n%s\n\n🔗 %s",
		dateText, content, newsItem.URL)

	// Use plain text mode
	msg := tgbotapi.NewMessageToChannel(fmt.Sprintf("%d", channelID), message)

	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("error sending message: %v", err)
	}

	return nil
}

func fetchWonderDaysNews() ([]NewsItem, error) {
	// Get the news listing page
	resp, err := http.Get(magticomNewsURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching news page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var wonderDaysItems []NewsItem

	// Find news items in the specified selector
	doc.Find("#article > article > div.post-listing.top-line > div > a").Each(func(i int, s *goquery.Selection) {
		url, exists := s.Attr("href")
		if exists {
			// Make relative URLs absolute
			if !strings.HasPrefix(url, "http") {
				url = "https://www.magticom.ge/" + strings.TrimPrefix(url, "/")
			}

			// Extract date from the post listing
			dateStr := s.Find("span.post-date").Text()

			newsItem := NewsItem{
				Title: strings.TrimSpace(s.Text()),
				URL:   url,
				Date:  dateStr,
			}

			// Fetch the full article content
			content, sectionContent, err := fetchArticleContent(url)
			if err != nil {
				log.Printf("Warning: couldn't fetch article content: %v", err)
			} else {
				newsItem.Text = content
				newsItem.SectionContent = sectionContent
			}

			wonderDaysItems = append(wonderDaysItems, newsItem)
		}
	})

	return wonderDaysItems, nil
}

// parseHtmlContent extracts text from HTML content with proper formatting
func parseHtmlContent(htmlContent string) string {
	reader := strings.NewReader(htmlContent)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		// If parsing fails, return the original content
		return htmlContent
	}

	var result strings.Builder

	// Process paragraphs and lists
	doc.Find("p, ul, li").Each(func(i int, s *goquery.Selection) {
		// Get the tag name
		tagName := goquery.NodeName(s)

		// Clean up the text
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		// Replace Georgian Lari icon with the proper symbol
		if s.Find("span.icon-gel").Length() > 0 {
			text = strings.TrimSpace(text) + " ₾"
		}

		switch tagName {
		case "p":
			// For paragraphs, add text followed by two newlines
			result.WriteString(text)
			result.WriteString("\n\n")
		case "ul":
			// Don't process ul directly, we'll handle li elements
		case "li":
			// For list items, add a bullet point
			result.WriteString("• ")
			result.WriteString(text)
			result.WriteString("\n")
		}
	})

	// Clean up the result
	content := result.String()

	// Replace HTML entities
	content = strings.ReplaceAll(content, "&nbsp;", " ")

	// Clean up any excess newlines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

func fetchArticleContent(url string) (string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", err
	}

	// Extract the date content
	dateContent := doc.Find("#article > article > div > div").Text()

	// Extract the section content as HTML
	sectionHtml, err := doc.Find("#article > article > div > section").Html()
	if err != nil {
		// If we can't get the HTML, fall back to text
		sectionContent := doc.Find("#article > article > div > section").Text()
		return strings.TrimSpace(dateContent), strings.TrimSpace(sectionContent), nil
	}

	// Parse the HTML content
	sectionContent := parseHtmlContent(sectionHtml)

	return strings.TrimSpace(dateContent), sectionContent, nil
}

func printInstructions() {
	fmt.Println("Missing required configuration for the Magticom News Reposter.")
	fmt.Println("\nPlease create the following configuration files:")

	// Secrets file
	fmt.Println("\n1. Secrets file (for the bot token):")
	fmt.Printf("   Path: %s\n", getDefaultSecretsPath())
	fmt.Println("   Format:")
	fmt.Println(`   {
     "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN"
     }`)
	fmt.Println("   To obtain, create a Telegram bot by talking to @BotFather and get the token")

	// Settings file
	fmt.Println("\n2. Settings file (for the channel ID):")
	fmt.Printf("   Path: %s\n", getSettingsPath())
	fmt.Println("   Format:")
	fmt.Println(`   {
     "telegram_channel_id": YOUR_CHANNEL_ID_NUMBER
   }`)
	fmt.Println("   To get it, add your bot to the target channel as an administrator,")
	fmt.Println("   and forward a message from the channel to @userinfobot.")
	fmt.Println("   Use the 'Id' number from the 'Forwarded from chat' value (including the negative sign)")
}

// Run executes the service
func Run() {
	secrets, err := loadSecrets()
	if err != nil {
		log.Printf("Error loading secrets: %v", err)
		printInstructions()
		return
	}

	settings, err := loadSettings()
	if err != nil {
		log.Printf("Error loading settings: %v", err)
		printInstructions()
		return
	}

	lastPostData, err := loadLastPostData()
	if err != nil {
		log.Printf("Error loading last post data: %v", err)
		// Continue anyway, we'll create new data
		lastPostData = &LastPostData{}
	}

	log.Println("Fetching Wonder Days news from Magticom...")
	news, err := fetchWonderDaysNews()
	if err != nil {
		log.Fatalf("Error fetching news: %v", err)
	}

	if len(news) == 0 {
		log.Println("No Wonder Days news found")
		return
	}

	// Since news are in order from newest to oldest,
	// we'll post all new items until we find the last posted URL

	var newItems []NewsItem
	var lastPostURL string
	foundLastPost := false

	// First, find all new items up to the previously posted one
	for _, item := range news {
		if item.URL == lastPostData.LastPostURL {
			foundLastPost = true
			break
		}
		newItems = append(newItems, item)
	}

	// If there are new items to post
	if len(newItems) > 0 {
		log.Printf("Found %d new Wonder Days items to post", len(newItems))

		// Remember the newest URL to save later
		lastPostURL = newItems[0].URL

		// Process items from oldest to newest (reverse the slice)
		for i := len(newItems) - 1; i >= 0; i-- {
			item := newItems[i]

			log.Printf("Posting to Telegram: %s", item.Title)

			if err := sendToTelegram(secrets.TelegramBotToken, settings.TelegramChannelID, item); err != nil {
				log.Printf("Error sending to Telegram: %v", err)
				continue
			}

			log.Printf("Successfully posted: %s", item.URL)
		}

		// Save the newest posted URL
		lastPostData.LastPostURL = lastPostURL
		if err := saveLastPostData(lastPostData); err != nil {
			log.Printf("Warning: couldn't update last post URL: %v", err)
		}
	} else if !foundLastPost && lastPostData.LastPostURL != "" {
		// If we didn't find the last post but we have one saved,
		// it might mean the content structure has changed or the post was removed
		// In this case, post the newest item and update the lastPostURL
		log.Printf("Could not find last posted URL (%s). Posting the newest item.", lastPostData.LastPostURL)
		if len(news) > 0 {
			newestItem := news[0]
			log.Printf("Posting to Telegram: %s", newestItem.Title)

			if err := sendToTelegram(secrets.TelegramBotToken, settings.TelegramChannelID, newestItem); err != nil {
				log.Printf("Error sending to Telegram: %v", err)
			} else {
				log.Printf("Successfully posted: %s", newestItem.URL)

				// Update the last post URL
				lastPostData.LastPostURL = newestItem.URL
				if err := saveLastPostData(lastPostData); err != nil {
					log.Printf("Warning: couldn't update last post URL: %v", err)
				}
			}
		}
	} else {
		log.Println("No new Wonder Days news to post")
	}
}

func main() {
	Run()
}
