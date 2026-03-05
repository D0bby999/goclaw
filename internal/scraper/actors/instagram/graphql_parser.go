package instagram

import (
	"strconv"
	"time"

	"github.com/tidwall/gjson"
)

// parseProfileFromGraphQL extracts an InstagramProfile from a GraphQL JSON response.
// Supports both legacy (data.user) and new XDT (data.xdt_api__v1__users__web_profile_info) paths.
func parseProfileFromGraphQL(body string) *InstagramProfile {
	for _, path := range []string{
		"data.xdt_api__v1__users__web_profile_info__connection.user",
		"data.xdt_api__v1__users__web_profile_info.user",
		"data.user",
	} {
		u := gjson.Get(body, path)
		if !u.Exists() || u.Get("username").String() == "" {
			continue
		}
		return &InstagramProfile{
			UserID:        firstString(u, "pk", "id"),
			Username:      u.Get("username").String(),
			FullName:      firstString(u, "full_name", "fullName"),
			Bio:           firstString(u, "biography", "bio"),
			ProfilePicURL: firstString(u, "profile_pic_url_hd", "profile_pic_url"),
			ExternalURL:   u.Get("external_url").String(),
			Followers:     firstInt(u, "follower_count", "edge_followed_by.count"),
			Following:     firstInt(u, "following_count", "edge_follow.count"),
			PostCount:     firstInt(u, "media_count", "edge_owner_to_timeline_media.count"),
			IsPrivate:     u.Get("is_private").Bool(),
			IsVerified:    u.Get("is_verified").Bool(),
		}
	}
	return nil
}

// parsePostsFromGraphQL extracts posts from a GraphQL JSON response.
// Supports both legacy edge format and new XDT timeline format.
func parsePostsFromGraphQL(body string) []InstagramPost {
	var posts []InstagramPost

	for _, path := range []string{
		"data.xdt_api__v1__feed__user_timeline_graphql_connection.edges",
		"data.user.edge_owner_to_timeline_media.edges",
		"data.xdt_api__v1__feed__timeline__connection.edges",
	} {
		edges := gjson.Get(body, path)
		if !edges.Exists() {
			continue
		}
		edges.ForEach(func(_, edge gjson.Result) bool {
			node := edge.Get("node")
			if !node.Exists() {
				node = edge
			}
			if p := parsePostNode(node); p.ID != "" || p.Shortcode != "" {
				posts = append(posts, p)
			}
			return true
		})
		if len(posts) > 0 {
			return posts
		}
	}

	// Also try single post detail paths
	for _, path := range []string{
		"data.xdt_api__v1__media__details",
		"data.xdt_api__v1__media__shortcode__web_info.media",
		"data.shortcode_media",
	} {
		node := gjson.Get(body, path)
		if node.Exists() {
			if p := parsePostNode(node); p.ID != "" || p.Shortcode != "" {
				posts = append(posts, p)
			}
		}
	}
	return posts
}

func parsePostNode(node gjson.Result) InstagramPost {
	shortcode := firstString(node, "code", "shortcode")
	postURL := ""
	if shortcode != "" {
		postURL = "https://www.instagram.com/p/" + shortcode + "/"
	}

	caption := node.Get("caption.text").String()
	if caption == "" {
		caption = node.Get("edge_media_to_caption.edges.0.node.text").String()
	}

	mediaType := resolveMediaType(node)
	isVideo := mediaType == "Video"

	ts := firstInt64(node, "taken_at", "taken_at_timestamp")
	timestamp := ""
	if ts > 0 {
		timestamp = time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}

	return InstagramPost{
		ID:            firstString(node, "pk", "id"),
		Shortcode:     shortcode,
		URL:           postURL,
		Caption:       caption,
		MediaURL:      firstString(node, "image_versions2.candidates.0.url", "display_url"),
		MediaType:     mediaType,
		Likes:         firstInt(node, "like_count", "edge_liked_by.count", "edge_media_preview_like.count"),
		Comments:      firstInt(node, "comment_count", "edge_media_to_comment.count"),
		Timestamp:     timestamp,
		OwnerUsername: firstString(node, "user.username", "owner.username"),
		OwnerID:       firstString(node, "user.pk", "owner.id"),
		IsVideo:       isVideo,
		VideoURL:      firstString(node, "video_versions.0.url", "video_url"),
	}
}

// resolveMediaType maps Instagram's media_type number or __typename to a string.
func resolveMediaType(node gjson.Result) string {
	if mt := node.Get("media_type").Int(); mt > 0 {
		switch mt {
		case 1:
			return "Image"
		case 2:
			return "Video"
		case 8:
			return "Sidecar"
		}
	}
	switch node.Get("__typename").String() {
	case "GraphImage", "XDTGraphImage":
		return "Image"
	case "GraphVideo", "XDTGraphVideo":
		return "Video"
	case "GraphSidecar", "XDTGraphSidecar":
		return "Sidecar"
	}
	if pt := node.Get("product_type").String(); pt != "" {
		switch pt {
		case "clips", "feed_reel":
			return "Video"
		case "carousel_container":
			return "Sidecar"
		}
	}
	return "Image"
}

// firstString returns the first non-empty string from the given gjson paths.
func firstString(r gjson.Result, paths ...string) string {
	for _, p := range paths {
		if v := r.Get(p).String(); v != "" {
			return v
		}
	}
	return ""
}

// firstInt returns the first non-zero int from the given gjson paths.
func firstInt(r gjson.Result, paths ...string) int {
	for _, p := range paths {
		if v := r.Get(p).Int(); v != 0 {
			return int(v)
		}
	}
	return 0
}

// firstInt64 returns the first non-zero int64 from the given gjson paths.
func firstInt64(r gjson.Result, paths ...string) int64 {
	for _, p := range paths {
		if v := r.Get(p).Int(); v != 0 {
			return v
		}
		// Also try string-encoded numbers
		if s := r.Get(p).String(); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}
