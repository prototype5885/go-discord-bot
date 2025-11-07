package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var conversation Conversation = *newConversation()
var url string

var nextKeyIndex int = 0
var keys []string

func setNextKey() {
	nextKeyIndex = (nextKeyIndex + 1) % len(keys)
}

func setUrl() {
	fmt.Printf("Next api key index is %d...\n", nextKeyIndex)
	if keys[nextKeyIndex] != "" {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/gemini-2.5-flash:generateContent?key=%s", keys[nextKeyIndex])
		setNextKey()
	} else {
		setNextKey()
		setUrl()
	}
}

func revertLastUserMsg() {
	if len(conversation.Contents) < 1 {
		return
	}

	log.Println("Reverting last user msg")
	conversation.Contents = conversation.Contents[:len(conversation.Contents)-1]
}

func revertLastTwoMsg() {
	if len(conversation.Contents) < 2 {
		return
	}

	log.Println("Reverting both last user and gemini msgs")
	conversation.Contents = conversation.Contents[:len(conversation.Contents)-2]
}

func main() {
	log.Println("Starting discord bot...")
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading .env file")
	}

	s, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		log.Fatalln(err)
		return
	}

	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m)
	})

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		ready(s, r)
	})

	s.Identify.Intents = discordgo.IntentsGuildMessages

	err = s.Open()
	if err != nil {
		log.Fatalln(err)
		return
	}

	keys = []string{}
	for i := range 16 {
		key := os.Getenv(fmt.Sprintf("GEMINI_API_KEY%d", i))
		if key != "" {
			keys = append(keys, key)
		}

	}
	fmt.Printf("Found %d gemini keys\n", len(keys))

	setUrl()

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	s.Close()
}

func ready(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Bot [%s] is connected!\n", r.User.Username)

	if err := s.UpdateCustomStatus("Tokens: 0"); err != nil {
		log.Println(err)
		return
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	var botID string = s.State.User.ID
	var senderID string = m.Author.ID

	if senderID == botID {
		return
	}

	if len(m.Content) <= 0 {
		log.Println("Received message is empty")
		return
	}

	if m.Content == "!restart" {
		os.Exit(0)
		return
	}

	if m.Content == "!resetgemini" {
		if err := s.UpdateCustomStatus("Tokens: 0"); err != nil {
			log.Println(err)
			return
		}
		if err := s.ChannelTyping(m.ChannelID); err != nil {
			log.Println(err)
			return
		}
		conversation = *newConversation()
		_, err := s.ChannelMessageSend(m.ChannelID, "Cleared gemini conversation!")
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Cleared gemini conversation")
		return
	}

	if m.Content == "!revert" || m.Content == "!undo" {
		revertLastTwoMsg()

		lastMessage := conversation.Contents[len(conversation.Contents)-1].Parts.Text
		var partOfMessage string
		if len(lastMessage) > 128 {
			partOfMessage = lastMessage[:128]
		} else {
			partOfMessage = lastMessage
		}

		_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Deleted last 2 messages! Current last message begins with:\n\n%s...", partOfMessage))
		if err != nil {
			log.Println(err)
			return
		}
		return
	}

	// checks if mentioned
	var mentioned bool = false
	for i := 0; i < len(m.Mentions); i++ {
		if m.Mentions[i].ID == botID {
			mentioned = true
			break
		}
	}

	// check if starts with question mark
	var questionMark bool = false
	if strings.HasPrefix(m.Content, "? ") {
		questionMark = true
	}

	// check if has attachments
	var hasAttachments bool = false
	if len(m.Attachments) > 0 && strings.HasPrefix(m.Content, "?") {
		hasAttachments = true
	}

	if mentioned || questionMark || hasAttachments {
		// removes mention from message
		var mention string = fmt.Sprintf("<@%s>", s.State.User.ID)
		var noMentionMsg string = strings.Replace(m.Content, mention, "", 1)

		if len(noMentionMsg) > 0 && noMentionMsg[0] == ' ' {
			noMentionMsg = noMentionMsg[1:]
		} else if len(noMentionMsg) > 1 && questionMark {
			// Step 3: If a question mark is true, remove the first two characters
			noMentionMsg = noMentionMsg[2:]
		}

		// sends typing indicator thing to discord
		err := s.ChannelTyping(m.ChannelID)
		if err != nil {
			log.Println(err)
			return
		}

		// sets these values to default, will be used later if there is image attachment
		var base64 string
		var contentType string = "text"

		// checks if there is attachment and grabs the first one
		if len(m.Attachments) > 0 {
			log.Printf("Attachment found: %s\n", m.Attachments[0].Filename)
			// check if attachment is in supported format
			var supported bool = false
			for _, item := range []string{"image/jpg", "image/jpeg", "image/png", "image/webp"} {
				if m.Attachments[0].ContentType == item {
					supported = true
					break
				}
			}
			if !supported {
				log.Println("Unsupported attachment type")
				_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Unsupported attachment type: [%s]", m.Attachments[0].ContentType))
				if err != nil {
					log.Println(err)
				}
				return
			}

			base64, err = downloadFile(m.Attachments[0])
			if err != nil {
				log.Println(err)
				return
			}
			contentType = m.Attachments[0].ContentType

			log.Printf("Size of base64 is: [%d]\n", len(base64))
		}

		log.Println("Forwarding message to gemini...")
		user_content := Contents{
			Role:  "user",
			Parts: Parts{Text: noMentionMsg},
		}

		log.Println("adding user's message to conversation...")
		conversation.Contents = append(conversation.Contents, user_content)

		log.Printf("Content type is: [%s]\n", contentType)

		var jsonBytes []byte
		if contentType == "text" {
			jsonBytes, err = json.Marshal(conversation)
			if err != nil {
				revertLastUserMsg()
				log.Fatal(err)
				return
			}
		} else {
			jsonBytes = []byte(fmt.Sprintf(`
					{
			            "contents": {
			                "role": "user",
			                "parts": [
			                    {
			                        "inlineData": {
			                            "data": "%s",
			                            "mimeType": "%s"
			                        }
			                    },
			                    {
			                        "text": "%s"
			                    }
			                ]
			            },
			            "safety_settings": [
			                {
			                    "category": "HARM_CATEGORY_SEXUALLY_EXPLICIT",
			                    "threshold": "BLOCK_NONE"
			                },
			                {
			                    "category": "HARM_CATEGORY_HATE_SPEECH",
			                    "threshold": "BLOCK_NONE"
			                },
			                {
			                    "category": "HARM_CATEGORY_HARASSMENT",
			                    "threshold": "BLOCK_NONE"
			                },
			                {
			                    "category": "HARM_CATEGORY_DANGEROUS_CONTENT",
			                    "threshold": "BLOCK_NONE"
			            }
			            ]
			        }`, base64, contentType, noMentionMsg))

		}

		log.Printf("Size: %f kb\n", float64(len(jsonBytes))/1024.0)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
		if err != nil {
			revertLastUserMsg()
			log.Println(err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			revertLastUserMsg()
			log.Println(err)
			return
		}

		var response Response
		if err := json.Unmarshal(body, &response); err != nil {
			revertLastUserMsg()
			log.Println(err)
		}

		if len(response.Candidates) != 0 && response.Candidates[0].FinishReason == "STOP" {
			// if response was success
			log.Println("Successful response from gemini")
			geminiResponse := Contents{
				Role: "model",
				Parts: Parts{
					Text: response.Candidates[0].Content.Parts[0].Text,
				},
			}

			// log.Println("Adding gemini's reply to conversation...")
			conversation.Contents = append(conversation.Contents, geminiResponse)
			if len(conversation.Contents) >= 10 {
				conversation.Contents = conversation.Contents[len(conversation.Contents)-10:]
			}

			// split messages into chunks
			if len(geminiResponse.Parts.Text) > 2000 {
				var chunks []string
				text := geminiResponse.Parts.Text
				for i := 0; i < len(text); {
					end := i + 2000
					if end > len(text) {
						end = len(text)
					} else {
						if idx := strings.LastIndexByte(text[i:end], ' '); idx != -1 {
							end = i + idx
						}
					}
					chunks = append(chunks, text[i:end])
					for end < len(text) && text[end] == ' ' {
						end++
					}
					i = end
				}

				// send the chunks
				for _, chunk := range chunks {
					log.Println("chunk size:", len(chunk))
					_, err = s.ChannelMessageSend(m.ChannelID, chunk)
					if err != nil {
						revertLastTwoMsg()
						log.Println(err)
						return
					}
				}
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, geminiResponse.Parts.Text)
				if err != nil {
					revertLastTwoMsg()
					log.Println(err)
					return
				}
			}

			if response.UsageMetaData.TotalTokenCount != -1 {
				status := fmt.Sprintf("Tokens: %d", response.UsageMetaData.TotalTokenCount)
				if err := s.UpdateCustomStatus(status); err != nil {
					revertLastTwoMsg()
					log.Println(err)
					return
				}
			}

		} else if len(response.Candidates) != 0 && response.Candidates[0].FinishReason == "SAFETY" {
			// if safety trigger
			_, err = s.ChannelMessageSend(m.ChannelID, "https://i.imgur.com/DJqE6wq.jpeg")
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			issue := fmt.Sprintf("Error code: %d", response.Error.Code)
			log.Println(issue)
			log.Println(response.Error.Message)
			if response.Error.Code == 429 {
				setUrl()
				issue = fmt.Sprintf("%s, changing api key to index %d", issue, nextKeyIndex)
			}
			_, err = s.ChannelMessageSend(m.ChannelID, issue)
			if err != nil {
				log.Println(err)
				return
			}
			return
		}

	}
}

func ffmpeg(inputBytes []byte) ([]byte, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-i", "pipe:0",
		"-vframes", "1",
		"-c:v", "mjpeg",
		"-q:v", "90",
		"-f", "image2",
		"pipe:1",
	)

	// print ffmpeg result
	// cmd.Stderr = os.Stderr

	// this will send the input picture bytes to ffmpeg
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	// this will store the converted image result
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// start the command
	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	// send the input bytes
	_, err = stdin.Write(inputBytes)
	if err != nil {
		return nil, err
	}

	err = stdin.Close()
	if err != nil {
		return nil, err
	}

	// wait for it to finish
	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	// read the converted image bytes back
	resultBytes := stdout.Bytes()

	return resultBytes, nil
}

func downloadFile(msg *discordgo.MessageAttachment) (string, error) {
	// download image
	resp, err := http.Get(msg.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// read bytes
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	jpgBytes, err := ffmpeg(imageData)
	if err != nil {
		return "", err
	}

	// encode to base64
	return base64.StdEncoding.EncodeToString(jpgBytes), nil
}
