import json

with open('links.json', 'r') as f:
    data = json.load(f)

bad_domains = [
    'businessinsider', 'bloomberg', 'cnbc', 'nytimes', 'wsj', 'cnn', 
    'foxbusiness', 'forbes', 'reuters', 'theverge', 'engadget', 'techcrunch'
]

initial = len(data['links'])
filtered = []
for feed in data['links']:
    url = feed['url'].lower()
    if not any(bad in url for bad in bad_domains):
        filtered.append(feed)

data['links'] = filtered

with open('links.json', 'w') as f:
    json.dump(data, f, indent=4)

print(f"Removed {initial - len(filtered)} noisy feeds.")
