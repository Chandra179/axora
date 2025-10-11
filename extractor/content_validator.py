"""
Content Quality Validation Module
Validates extracted content quality before further processing
"""

import logging

logger = logging.getLogger(__name__)

# Quality thresholds
MIN_WORD_COUNT = 100
MIN_CONTENT_RATIO = 0.05  # 5% of HTML should be actual content
ALLOWED_LANGUAGES = ['en']  # English only


def validate_content_quality(extracted: dict) -> dict:
    """
    Validate if extracted content meets quality standards
    
    Args:   
        extracted: Dictionary from web_extractor.extract_content()
        
    Returns:
        dict: {
            "passed": bool,
            "reasons": list[str],
            "quality_score": float (0-100),
            "checks": dict with individual check results
        }
    """
    reasons = []
    checks = {}
    score = 100.0
    
    # Check 1: Minimum word count
    word_count = extracted.get('word_count', 0)
    if word_count < MIN_WORD_COUNT:
        reasons.append(f"Too short: {word_count} words (minimum: {MIN_WORD_COUNT})")
        score -= 40
        checks['word_count'] = False
    else:
        checks['word_count'] = True
        logger.info(f"✓ Word count: {word_count} words")
    
    # Check 2: Has title
    title = extracted.get('title', '')
    if not title or len(title.strip()) < 10:
        reasons.append("No valid title found")
        score -= 20
        checks['has_title'] = False
    else:
        checks['has_title'] = True
        logger.info(f"✓ Title: {title[:50]}...")
    
    # Check 3: Content-to-HTML ratio (detect if extraction got mostly noise)
    html_length = extracted.get('html_length', 0)
    char_count = extracted.get('char_count', 0)
    
    if html_length > 0:
        content_ratio = char_count / html_length
        
        if content_ratio < MIN_CONTENT_RATIO:
            reasons.append(f"Low content ratio: {content_ratio:.1%} (minimum: {MIN_CONTENT_RATIO:.1%})")
            score -= 30
            checks['content_ratio'] = False
        else:
            checks['content_ratio'] = True
            logger.info(f"✓ Content ratio: {content_ratio:.1%}")
    else:
        checks['content_ratio'] = False
        reasons.append("Could not calculate content ratio")
        score -= 10
    
    # Check 5: Has publication date (optional, but good indicator for news)
    date = extracted.get('date')
    if not date:
        reasons.append("No publication date found")
        score -= 10
        checks['has_date'] = False
    else:
        checks['has_date'] = True
        logger.info(f"✓ Publication date: {date}")
    
    # Ensure score doesn't go below 0
    score = max(0, score)
    
    # Pass if score >= 50
    passed = score >= 50
    
    if passed:
        logger.info(f"✓ Content quality passed (score: {score:.1f}/100)")
    else:
        logger.warning(f"✗ Content quality failed (score: {score:.1f}/100)")
        for reason in reasons:
            logger.warning(f"  - {reason}")
    
    return {
        "passed": passed,
        "reasons": reasons,
        "quality_score": score,
        "checks": checks
    }