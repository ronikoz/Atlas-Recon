#!/usr/bin/env python3
import argparse
import sys
import json
from datetime import datetime
import requests

try:
    from atproto import Client
except ImportError:
    print("atproto is required. Install with: pip install atproto", file=sys.stderr)
    sys.exit(1)

def fetch_author_feed(handle, limit=10):
    params = {
        'actor': handle,
        'limit': limit
    }
    try:
        resp = requests.get(
            "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed",
            params=params,
            timeout=10
        )
        resp.raise_for_status()
        data = resp.json()
        # format similar to search results for consistent parsing
        posts = []
        for p in data.get("feed", []):
            post_data = p.get("post", {})
            record = post_data.get("record", {})
            author = post_data.get("author", {})
            posts.append({
                "record": record,
                "author": author,
                "replyCount": post_data.get("replyCount", 0),
                "repostCount": post_data.get("repostCount", 0),
                "likeCount": post_data.get("likeCount", 0)
            })
        return {"posts": posts}
    except Exception as e:
         return {"error": f"Feed fetch failed: {e}"}

def search_bluesky(query, limit=10):
    # Search API (app.bsky.feed.searchPosts) often requires auth.
    # We will fallback to fetching an author's feed if the query looks like a handle.
    # Otherwise, we will try search but catch the 403 gracefully.

    if "." in query and not " " in query:
        # Likely a handle (e.g. user.bsky.social)
        return fetch_author_feed(query, limit)

    params = {
        'q': query,
        'limit': limit
    }
    
    try:
        # Try generic search
        resp = requests.get(
            "https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts", 
            params=params, 
            timeout=10
        )
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        return {"error": f"Search failed (Auth likely required for generic search). Try searching for a specific handle (e.g. @jay.bsky.team). Error: {e}"}

def main():
    parser = argparse.ArgumentParser(description="Social Pulse (Bluesky)")
    parser.add_argument("query", help="Keyword or Handle (e.g. @bsky.app)")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    parser.add_argument("--limit", type=int, default=10, help="Max posts")
    args = parser.parse_args()

    # Clean handle if starts with @
    query = args.query
    if query.startswith("@"):
        query = query[1:]

    data = search_bluesky(query, args.limit)
    
    # Process results
    posts = data.get("posts", [])
    
    results = {
        "query": query,
        "timestamp": datetime.now().isoformat(),
        "count": len(posts),
        "posts": []
    }

    for post in posts:
        record = post.get("record", {})
        author = post.get("author", {})
        
        clean_post = {
            "text": record.get("text", ""),
            "author_handle": author.get("handle"),
            "display_name": author.get("displayName"),
            "posted_at": record.get("createdAt"),
            "replies": post.get("replyCount", 0),
            "reposts": post.get("repostCount", 0),
            "likes": post.get("likeCount", 0),
        }
        results["posts"].append(clean_post)

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        if "error" in data:
             print(f"Error: {data['error']}", file=sys.stderr)
             print("Note: Bluesky public search is often restricted. Try a handle like 'bsky.app'.", file=sys.stderr)
             sys.exit(1)

        print(f"Social Pulse: {query}")
        print(f"Source: Bluesky (Public API)")
        print("--------------------------------------------------")
        
        if not results["posts"]:
            print("No recent posts found.")
        
        for p in results["posts"]:
            print(f"@{p['author_handle']} ({p['display_name']}):")
            print(f"  {p['text']}")
            print(f"  [Likes: {p['likes']} Reposts: {p['reposts']}]")
            print("")

if __name__ == "__main__":
    main()
