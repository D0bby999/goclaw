package facebook

import (
	"time"

	"github.com/tidwall/gjson"
)

// feedEdgePaths lists known GraphQL paths for Facebook feed post edges.
var feedEdgePaths = []string{
	"data.node.timeline_feed_units.edges",
	"data.node.timeline_list_feed_units.edges",
	"data.viewer.timeline_feed_connection.edges",
	"data.page.timeline_feed_units.edges",
}

// feedbackPaths lists known GraphQL paths for post engagement data.
var feedbackPaths = []string{
	"comet_sections.feedback.story.story_ufi_container.story.feedback_context.feedback_target_with_context.comet_ufi_summary_and_actions_renderer.feedback",
	"comet_sections.feedback.story.feedback_context.feedback_target_with_context.comet_ufi_summary_and_actions_renderer.feedback",
}

// parsePostsFromGraphQL extracts Facebook posts from a single GraphQL JSON object.
func parsePostsFromGraphQL(body string, inputURL, pageID string) []FacebookPost {
	var posts []FacebookPost

	for _, path := range feedEdgePaths {
		edges := gjson.Get(body, path)
		if !edges.Exists() || !edges.IsArray() {
			continue
		}
		edges.ForEach(func(_, edge gjson.Result) bool {
			node := edge.Get("node")
			if !node.Exists() {
				return true
			}
			if p := parseGraphQLNode(node, inputURL, pageID); p != nil {
				posts = append(posts, *p)
			}
			return true
		})
		if len(posts) > 0 {
			return posts
		}
	}

	return posts
}

// parseGraphQLNode extracts a FacebookPost from a GraphQL node.
func parseGraphQLNode(node gjson.Result, inputURL, pageID string) *FacebookPost {
	postID := node.Get("post_id").String()
	if postID == "" {
		postID = node.Get("id").String()
	}
	if postID == "" {
		return nil
	}

	// Timestamp
	createdTime := node.Get("comet_sections.timestamp.story.creation_time").Int()
	timestamp := ""
	if createdTime > 0 {
		timestamp = time.Unix(createdTime, 0).UTC().Format(time.RFC3339)
	}

	// URL
	url := node.Get("comet_sections.timestamp.story.url").String()
	if url == "" {
		url = "https://www.facebook.com/" + pageID + "/posts/" + postID
	}

	// Author from feedback.owning_profile
	authorID := node.Get("feedback.owning_profile.id").String()
	authorName := node.Get("feedback.owning_profile.name").String()
	if authorName == "" {
		authorName = pageID
	}
	authorURL := ""
	if authorID != "" {
		authorURL = "https://www.facebook.com/profile.php?id=" + authorID
	}

	// Text
	text := node.Get("comet_sections.content.story.comet_sections.message.story.message.text").String()

	// Engagement — try multiple feedback paths
	var fb gjson.Result
	for _, fp := range feedbackPaths {
		fb = node.Get(fp)
		if fb.Exists() {
			break
		}
	}

	likes := int(fb.Get("reaction_count.count").Int())
	shares := int(fb.Get("share_count.count").Int())

	// Comments — try two nested paths
	comments := int(fb.Get("comments_count_summary_renderer.feedback.comment_rendering_instance.comments.total_count").Int())
	if comments == 0 {
		comments = int(fb.Get("comment_rendering_instance.comments.total_count").Int())
	}

	// Images from attachments
	var imageURLs []string
	node.Get("attachments").ForEach(func(_, att gjson.Result) bool {
		media := att.Get("styles.attachment.media")
		if uri := media.Get("photo_image.uri").String(); uri != "" {
			imageURLs = append(imageURLs, uri)
		}
		// Subattachments (album)
		att.Get("styles.attachment.all_subattachments.nodes").ForEach(func(_, sub gjson.Result) bool {
			if uri := sub.Get("media.photo_image.uri").String(); uri != "" {
				imageURLs = append(imageURLs, uri)
			}
			return true
		})
		return true
	})

	return &FacebookPost{
		ID:         postID,
		Text:       text,
		AuthorName: authorName,
		AuthorURL:  authorURL,
		URL:        url,
		Timestamp:  timestamp,
		Likes:      likes,
		Comments:   comments,
		Shares:     shares,
		ImageURLs:  imageURLs,
	}
}
