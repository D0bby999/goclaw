package scraper

import (
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/ecommerce"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/facebook"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/google_search"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/google_trends"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/instagram"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/instagram_reel"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/reddit"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/tiktok"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/twitter"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/website"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actors/youtube"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// ActorFactory creates an actor from a generic input map and HTTP client.
type ActorFactory func(input map[string]any, client *httpclient.Client) (actor.Actor, error)

// actorFactories maps platform names to their constructor functions.
var actorFactories = map[string]ActorFactory{
	"reddit":         func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return reddit.NewActor(i, c) },
	"twitter":        func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return twitter.NewActor(i, c) },
	"tiktok":         func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return tiktok.NewActor(i, c) },
	"youtube":        func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return youtube.NewActor(i, c) },
	"instagram":      func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return instagram.NewActor(i, c) },
	"instagram_reel": func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return instagram_reel.NewActor(i, c) },
	"facebook":       func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return facebook.NewActor(i, c) },
	"google_search":  func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return google_search.NewActor(i, c) },
	"google_trends":  func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return google_trends.NewActor(i, c) },
	"ecommerce":      func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return ecommerce.NewActor(i, c) },
	"website":        func(i map[string]any, c *httpclient.Client) (actor.Actor, error) { return website.NewActor(i, c) },
}

// CreateActor looks up the factory for actorName and constructs the actor.
func CreateActor(actorName string, input map[string]any, client *httpclient.Client) (actor.Actor, error) {
	factory, ok := actorFactories[actorName]
	if !ok {
		return nil, fmt.Errorf("unknown actor %q, supported: reddit, twitter, tiktok, youtube, instagram, instagram_reel, facebook, google_search, google_trends, ecommerce, website", actorName)
	}
	return factory(input, client)
}

// SupportedActors returns the list of registered actor names.
func SupportedActors() []string {
	names := make([]string, 0, len(actorFactories))
	for name := range actorFactories {
		names = append(names, name)
	}
	return names
}
