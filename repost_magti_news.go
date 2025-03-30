package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Secrets holds the configuration for the bot
type Secrets struct {
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChannelID int64 `json:"telegram_channel_id"`
	LastPostURL       string `json:"last_post_url,omitempty"`
}

// NewsItem represents a news item
type NewsItem struct {
	Title string
	URL   string
	Date  string
	Text  string
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

func loadSecrets() (*Secrets, error) {
	secretsPath := getDefaultSecretsPath()
	
	// Check if file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("secrets file not found at %s", secretsPath)
	}
	
	data, err := ioutil.ReadFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading secrets file: %v", err)
	}
	
	var secrets Secrets
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("error parsing secrets file: %v", err)
	}
	
	if secrets.TelegramBotToken == "" || secrets.TelegramChannelID == 0 {
		return nil, fmt.Errorf("missing required secrets")
	}
	
	return &secrets, nil
}

func saveSecrets(secrets *Secrets) error {
	secretsPath := getDefaultSecretsPath()
	
	// Ensure directory exists
	dir := filepath.Dir(secretsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}
	
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling secrets: %v", err)
	}
	
	if err := ioutil.WriteFile(secretsPath, data, 0600); err != nil {
		return fmt.Errorf("error writing secrets file: %v", err)
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
		title := strings.TrimSpace(s.Text())
		
		// Check if this is a "Wonder Days" post
		if strings.Contains(title, "Wonder Days") {
			url, exists := s.Attr("href")
			if exists {
				// Make relative URLs absolute
				if !strings.HasPrefix(url, "http") {
					url = "https://www.magticom.ge/" + strings.TrimPrefix(url, "/")
				}
				
				// Extract date from the post listing
				dateStr := s.Find("span.post-date").Text()
				
				newsItem := NewsItem{
					Title: "Wonder Days",
					URL:   url,
					Date:  dateStr,
				}
				
				// Fetch the full article content
				fullContent, err := fetchArticleContent(url)
				if err != nil {
					log.Printf("Warning: couldn't fetch article content: %v", err)
				} else {
					newsItem.Text = fullContent
				}
				
				wonderDaysItems = append(wonderDaysItems, newsItem)
			}
		}
	})
	
	return wonderDaysItems, nil
}

func fetchArticleContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}
	
	// Extract the article content
	content := doc.Find("#article > article > div > div").Text()
	return strings.TrimSpace(content), nil
}

func sendToTelegram(botToken string, channelID int64, newsItem NewsItem) error {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return fmt.Errorf("error initializing bot: %v", err)
	}
	
	// Compose message
	message := fmt.Sprintf("📢 *Wonder Days at Magticom*\n\n%s\n\n🔗 [Read more](%s)", 
		newsItem.Text, newsItem.URL)
	
	msg := tgbotapi.NewMessageToChannel(fmt.Sprintf("%d", channelID), message)
	msg.ParseMode = "Markdown"
	
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("error sending message: %v", err)
	}
	
	return nil
}

func printInstructions() {
	fmt.Println("Missing required configuration.")
	fmt.Println("\nPlease create a JSON configuration file at one of these locations:")
	fmt.Printf("- Path specified by SECRETS_PATH environment variable\n")
	fmt.Printf("- $SecretDataDir/repost_magti_news.json\n")
	fmt.Printf("- %s (default)\n", getDefaultSecretsPath())
	
	fmt.Println("\nThe file should have the following format:")
	fmt.Println(`{
  "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN",
  "telegram_channel_id": YOUR_CHANNEL_ID_NUMBER
}`)
	
	fmt.Println("\nTo obtain these values:")
	fmt.Println("1. Create a Telegram bot by talking to @BotFather and get the token")
	fmt.Println("2. Add your bot to the target channel as an administrator")
	fmt.Println("3. Get your channel ID by forwarding a message from the channel to @userinfobot")
	fmt.Println("   and using the number in the 'Forwarded from chat' value (including the negative sign)")
}

// Run executes the service
func Run() {
	secrets, err := loadSecrets()
	if err != nil {
		log.Printf("Error loading secrets: %v", err)
		printInstructions()
		return
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
	
	// Process each news item, latest first
	for i := len(news) - 1; i >= 0; i-- {
		item := news[i]
		
		// Skip if we already posted this
		if secrets.LastPostURL == item.URL {
			log.Printf("Skipping already posted item: %s", item.URL)
			continue
		}
		
		log.Printf("Posting to Telegram: %s", item.Title)
		
		if err := sendToTelegram(secrets.TelegramBotToken, secrets.TelegramChannelID, item); err != nil {
			log.Printf("Error sending to Telegram: %v", err)
			continue
		}
		
		// Update last posted URL
		secrets.LastPostURL = item.URL
		if err := saveSecrets(secrets); err != nil {
			log.Printf("Warning: couldn't update last post URL: %v", err)
		}
		
		log.Printf("Successfully posted: %s", item.URL)
	}
}

func main() {
	Run()
}

