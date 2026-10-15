import unittest

from textutils import slugify, word_count


class TextUtilsTests(unittest.TestCase):
    def test_slugify_basic(self):
        self.assertEqual(slugify("Hello World"), "hello-world")

    def test_slugify_strips_edges(self):
        self.assertEqual(slugify("  Hello, World!  "), "hello-world")

    def test_slugify_collapses_runs(self):
        self.assertEqual(slugify("a -- b"), "a-b")

    def test_word_count_mixed_whitespace(self):
        self.assertEqual(word_count("a  b\tc\nd"), 4)

    def test_word_count_blank(self):
        self.assertEqual(word_count("   "), 0)


if __name__ == "__main__":
    unittest.main()
