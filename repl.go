package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

// cliCommand yapısı komut bilgilerini tutar.
type cliCommand struct {
	name        string
	description string
	callback    func(*Cache, []string) error
}

// Komutları tutan bir harita (registry)
var commandRegistry map[string]cliCommand

func init() {
	commandRegistry = map[string]cliCommand{
		"exit": {
			name:        "exit",
			description: "Exit the Pokedex",
			callback:    commandExit,
		},
		"help": {
			name:        "help",
			description: "Displays a help message",
			callback:    commandHelp,
		},
		"map": {
			name:        "map",
			description: "Get Pokemon maps",
			callback:    commandMap,
		},
		"explore": {
			name:        "explore",
			description: "Explore a specific location area",
			callback:    commandExplore,
		},
		"catch": {
			name:        "catch",
			description: "Try to catch a Pokémon by name",
			callback:    commandCatch,
		},
		"pokedex": {
			name:        "pokedex",
			description: "List all caught Pokémon",
			callback:    commandPokedex,
		},
	}
}

func startRepl() {
	cache := NewCache()
	reader := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Pokedex > ")
		reader.Scan()

		words := cleanInput(reader.Text())
		if len(words) == 0 {
			continue
		}

		commandName := words[0]
		args := words[1:]

		command, exists := commandRegistry[commandName]
		if exists {
			if err := command.callback(cache, args); err != nil {
				fmt.Println("Error:", err)
			}
		} else {
			fmt.Println("Unknown command:", commandName)
		}
	}
}

// Kullanıcı girişini temizleyen fonksiyon
func cleanInput(text string) []string {
	output := strings.ToLower(text)
	words := strings.Fields(output)
	return words
}

func commandExit(cache *Cache, args []string) error {
	fmt.Println("Closing the Pokedex... Goodbye!")
	os.Exit(0)
	return nil
}

func commandHelp(cache *Cache, args []string) error {
	fmt.Println("Welcome to the Pokedex!")
	fmt.Println("Usage:")
	for _, cmd := range commandRegistry {
		fmt.Printf("%s: %s\n", cmd.name, cmd.description)
	}
	return nil
}

// map komutu
func commandMap(cache *Cache, args []string) error {
	url := "https://pokeapi.co/api/v2/location-area/"
	cachedData, found := cache.Get(url)
	if found {
		fmt.Println("Using cached data:")
		fmt.Println(string(cachedData))
		return nil
	}

	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response: %v", err)
	}

	cache.Add(url, body)

	fmt.Println("Fetched from API:")
	fmt.Println(string(body))
	return nil
}

func commandExplore(cache *Cache, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Please provide a location area to explore")
	}

	areaName := args[0]
	url := fmt.Sprintf("https://pokeapi.co/api/v2/location-area/%s/", areaName)

	// Önce cache'den kontrol et
	cachedData, found := cache.Get(url)
	if found {
		fmt.Printf("Exploring %s...\n", areaName)
		return printPokemonFromResponse(cachedData)
	}

	// API'den veri al
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to fetch location area: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response: %v", err)
	}

	// Cache'e ekle
	cache.Add(url, body)

	fmt.Printf("Exploring %s...\n", areaName)
	return printPokemonFromResponse(body)
}

func printPokemonFromResponse(data []byte) error {
	var result struct {
		PokemonEncounters []struct {
			Pokemon struct {
				Name string `json:"name"`
			} `json:"pokemon"`
		} `json:"pokemon_encounters"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("Failed to parse response: %v", err)
	}

	fmt.Println("Found Pokemon:")
	for _, encounter := range result.PokemonEncounters {
		fmt.Printf("- %s\n", encounter.Pokemon.Name)
	}
	return nil
}

type PokemonInfo struct {
	Height int
	Weight int
	Stats  map[string]int
	Types  []string
}

var caughtPokemon = make(map[string]PokemonInfo)

func commandCatch(cache *Cache, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Please provide a Pokémon name to catch")
	}

	pokemonName := strings.ToLower(args[0])
	if _, found := caughtPokemon[pokemonName]; found {
		fmt.Printf("%s is already caught!\n", pokemonName)
		return nil
	}

	fmt.Printf("Throwing a Pokeball at %s...\n", pokemonName)

	// API'den Pokémon bilgilerini çek
	url := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%s", pokemonName)
	cachedData, found := cache.Get(url)
	var data []byte
	if found {
		data = cachedData
	} else {
		res, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("Failed to fetch Pokémon data: %v", err)
		}
		defer res.Body.Close()

		data, err = io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("Failed to read response: %v", err)
		}
		cache.Add(url, data)
	}

	// JSON yanıtını çözümle
	var pokemon struct {
		Height         int `json:"height"`
		Weight         int `json:"weight"`
		BaseExperience int `json:"base_experience"`
		Stats          []struct {
			Stat struct {
				Name string `json:"name"`
			} `json:"stat"`
			BaseStat int `json:"base_stat"`
		} `json:"stats"`
		Types []struct {
			Type struct {
				Name string `json:"name"`
			} `json:"type"`
		} `json:"types"`
	}
	if err := json.Unmarshal(data, &pokemon); err != nil {
		return fmt.Errorf("Failed to parse Pokémon data: %v", err)
	}

	// Yakalama olasılığı hesapla
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	catchChance := r.Intn(100)
	if catchChance < 100-pokemon.BaseExperience {
		fmt.Printf("%s was caught!\n", pokemonName)

		// Pokémon bilgilerini kaydet
		stats := make(map[string]int)
		for _, stat := range pokemon.Stats {
			stats[stat.Stat.Name] = stat.BaseStat
		}

		types := []string{}
		for _, t := range pokemon.Types {
			types = append(types, t.Type.Name)
		}

		caughtPokemon[pokemonName] = PokemonInfo{
			Height: pokemon.Height,
			Weight: pokemon.Weight,
			Stats:  stats,
			Types:  types,
		}
	} else {
		fmt.Printf("%s escaped!\n", pokemonName)
	}

	return nil
}

func commandInspect(cache *Cache, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Please provide a Pokémon name to inspect")
	}

	pokemonName := strings.ToLower(args[0])

	// Pokémon yakalanmış mı kontrol et
	info, found := caughtPokemon[pokemonName]
	if !found {
		fmt.Println("You have not caught that Pokémon.")
		return nil
	}

	// Bilgileri yazdır
	fmt.Printf("Name: %s\n", pokemonName)
	fmt.Printf("Height: %d\n", info.Height)
	fmt.Printf("Weight: %d\n", info.Weight)
	fmt.Println("Stats:")
	for stat, value := range info.Stats {
		fmt.Printf("  - %s: %d\n", stat, value)
	}
	fmt.Println("Types:")
	for _, t := range info.Types {
		fmt.Printf("  - %s\n", t)
	}
	return nil
}

func commandPokedex(cache *Cache, args []string) error {
	if len(caughtPokemon) == 0 {
		fmt.Println("You have not caught any Pokémon yet.")
		return nil
	}

	fmt.Println("Your Pokedex:")
	for name := range caughtPokemon {
		fmt.Printf("- %s\n", name)
	}
	return nil
}
