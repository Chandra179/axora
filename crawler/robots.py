"""
robots.txt related process
"""
import re
import urllib.parse
from typing import Dict, List, Optional
from urllib.robotparser import RobotFileParser
import httpx
from datetime import datetime, timezone
import logging

logger = logging.getLogger(__name__)


class RobotsChecker:
    def __init__(self, cache_ttl_seconds: int = 3600):
        self.cache: Dict[str, Dict] = {}
        self.cache_ttl = cache_ttl_seconds
        
    def _get_robots_url(self, url: str) -> str:
        parsed = urllib.parse.urlparse(url)
        return f"{parsed.scheme}://{parsed.netloc}/robots.txt"
    
    def _is_cache_valid(self, domain: str) -> bool:
        if domain not in self.cache:
            return False
        cache_entry = self.cache[domain]
        cache_time = cache_entry.get('timestamp')
        if not cache_time:
            return False
        
        now = datetime.now(timezone.utc)
        return (now - cache_time).total_seconds() < self.cache_ttl
    
    async def _fetch_robots_txt(self, robots_url: str) -> str:
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                response = await client.get(robots_url)
                if response.status_code == 200:
                    return response.text
                elif response.status_code in [404, 403]:
                    logger.info(f"No robots.txt found at {robots_url} (status: {response.status_code}), allowing crawling")
                    return ""
                else:
                    logger.warning(f"Failed to fetch {robots_url}, status: {response.status_code}, allowing crawling")
                    return ""
        except Exception as e:
            logger.info(f"Error fetching robots.txt from {robots_url}: {e}, allowing crawling")
            return ""
    
    def _parse_robots_txt(self, content: str, user_agent: str = "*") -> Dict[str, any]:
        if not content.strip():
            return {
                'allowed': True,
                'disallowed_paths': [],
                'crawl_delay': None,
                'sitemap_urls': []
            }
        
        rp = RobotFileParser()
        rp.set_url("dummy")
        
        disallowed_paths = []
        crawl_delay = None
        sitemap_urls = []
        
        lines = content.split('\n')
        in_relevant_section = False
        
        for line in lines:
            line = line.strip()
            if not line or line.startswith('#'):
                continue
                
            if line.lower().startswith('user-agent:'):
                ua = line.split(':', 1)[1].strip()
                in_relevant_section = (ua == '*' or ua.lower() == user_agent.lower())
                
            elif line.lower().startswith('disallow:') and in_relevant_section:
                path = line.split(':', 1)[1].strip()
                if path:
                    disallowed_paths.append(path)
                    
            elif line.lower().startswith('crawl-delay:') and in_relevant_section:
                try:
                    crawl_delay = float(line.split(':', 1)[1].strip())
                except ValueError:
                    pass
                    
            elif line.lower().startswith('sitemap:'):
                sitemap_url = line.split(':', 1)[1].strip()
                if sitemap_url:
                    sitemap_urls.append(sitemap_url)
        
        return {
            'allowed': len(disallowed_paths) == 0 or not any(path == '/' for path in disallowed_paths),
            'disallowed_paths': disallowed_paths,
            'crawl_delay': crawl_delay,
            'sitemap_urls': sitemap_urls,
            'raw_content': content
        }
    
    async def check_url_allowed(self, url: str, user_agent: str = "*") -> Dict[str, any]:
        parsed = urllib.parse.urlparse(url)
        domain = parsed.netloc
        path = parsed.path or '/'
        
        if self._is_cache_valid(domain):
            robots_data = self.cache[domain]['data']
        else:
            robots_url = self._get_robots_url(url)
            robots_content = await self._fetch_robots_txt(robots_url)
            robots_data = self._parse_robots_txt(robots_content, user_agent)
            
            self.cache[domain] = {
                'data': robots_data,
                'timestamp': datetime.now(timezone.utc)
            }
        
        path_allowed = True
        blocking_rule = None
        
        for disallowed_path in robots_data['disallowed_paths']:
            if self._path_matches(path, disallowed_path):
                path_allowed = False
                blocking_rule = disallowed_path
                break
        
        return {
            'allowed': path_allowed,
            'reason': f"Blocked by rule: {blocking_rule}" if not path_allowed else "Allowed",
            'crawl_delay': robots_data.get('crawl_delay'),
            'sitemap_urls': robots_data.get('sitemap_urls', [])
        }
    
    def _path_matches(self, path: str, pattern: str) -> bool:
        if pattern == '/':
            return True
        
        if pattern.endswith('*'):
            return path.startswith(pattern[:-1])
        
        if '*' in pattern:
            regex_pattern = pattern.replace('*', '.*')
            return re.match(regex_pattern, path) is not None
        
        return path.startswith(pattern)
    
    def get_cached_domains(self) -> List[str]:
        return list(self.cache.keys())
    
    def clear_cache(self, domain: str = None):
        if domain:
            self.cache.pop(domain, None)
        else:
            self.cache.clear()


