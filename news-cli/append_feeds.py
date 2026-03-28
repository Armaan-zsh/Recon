import json
import urllib.request
import re

url = "https://raw.githubusercontent.com/kilimchoi/engineering-blogs/master/engineering_blogs.opml"
try:
    with urllib.request.urlopen(url) as response:
        content = response.read().decode('utf-8')
    
    # Extract url and title
    url_pattern = re.compile(r'xmlUrl="([^"]+)"')
    title_pattern = re.compile(r'title="([^"]+)"')
    
    new_feeds = []
    for line in content.split('\n'):
        u_match = url_pattern.search(line)
        t_match = title_pattern.search(line)
        if u_match:
            u = u_match.group(1)
            t = t_match.group(1) if t_match else "Engineering Blog"
            new_feeds.append({"name": t, "url": u})
            
    with open("links.json", "r") as f:
        data = json.load(f)
        
    existing_urls = {f["url"].lower().strip() for f in data["links"]}
    
    added = 0
    for feed in new_feeds:
        u = feed["url"].lower().strip()
        if u not in existing_urls:
            existing_urls.add(u)
            data["links"].append(feed)
            added += 1
            
    with open("links.json", "w") as f:
        json.dump(data, f, indent=4)
        
    print(f"Added {added} feeds. Total feeds now: {len(data['links'])}")
except Exception as e:
    print(f"Error: {e}")
