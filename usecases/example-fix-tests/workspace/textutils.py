import re


def slugify(text):
    """Lower-case the text and replace runs of non-alphanumerics with a single '-'.
    Leading and trailing '-' must be removed."""
    text = text.lower()
    text = re.sub(r"[^a-z0-9]+", "-", text)
    return text


def word_count(text):
    """Number of words separated by any whitespace. Empty or blank text has 0 words."""
    return len(text.split(" "))
