import json

with open('links.json', 'r') as f:
    data = json.load(f)

# Hardcore elitist sources
new_elite = [
    {"name": "Krebs on Security", "url": "https://krebsonsecurity.com/feed/"},
    {"name": "Phoronix (Linux)", "url": "https://www.phoronix.com/rss.php"},
    {"name": "The Register (Security)", "url": "https://www.theregister.com/security/headlines.atom"},
    {"name": "BleepingComputer", "url": "https://www.bleepingcomputer.com/feed/"},
    {"name": "The Hacker News", "url": "https://feeds.feedburner.com/TheHackersNews"},
    {"name": "Schneier on Security", "url": "https://www.schneier.com/blog/index.xml"},
    {"name": "Platformer (Platform Intelligence)", "url": "https://www.platformer.news/feed"},
    {"name": "Linux Kernel Newbies", "url": "https://kernelnewbies.org/LinuxChanges?action=rss_rc"},
    {"name": "Discord Developer Blog", "url": "https://discord.com/blog/rss.xml"}
]

# Blacklist of generic news/noisy sites
bad_names = ["ZDNet", "TechCrunch", "Business Insider", "Engadget", "The Verge", "CNET"]

# Filter out the junk
initial_count = len(data['links'])
filtered = []
existing_urls = {f['url'].lower().strip() for f in new_elite}

for feed in data['links']:
    name = feed.get('name', '')
    url = feed.get('url', '').lower()
    
    # Check if generic name
    if any(bad in name for bad in bad_names):
        continue
        
    # Check if we already have it in elite list (dedup)
    if url.strip() in existing_urls:
        continue
        
    filtered.append(feed)

# Add the new elites to the top
data['links'] = new_elite + filtered

with open('links.json', 'w') as f:
    json.dump(data, f, indent=4)

print(f"Purged {initial_count - len(filtered)} generic sources. Added {len(new_elite)} elite sources.")
print(f"Total feeds: {len(data['links'])}")
