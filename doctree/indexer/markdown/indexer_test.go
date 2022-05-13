package markdown

import (
	"testing"

	"github.com/hexops/autogold"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func Test_markdownToPage_simple(t *testing.T) {
	page := markdownToPage([]byte(`# ziglearn

Repo for https://ziglearn.org content. Feedback and PRs welcome.

## How to run the tests

1. `+"`"+`zig run test-out.zig`+"`"+`
2. `+"`"+`zig test do_tests.zig`+"`"+`
`), "README.md")

	autogold.Want("simple", schema.Page{
		Path: "README.md", Title: "ziglearn", Detail: schema.Markdown(`
Repo for https://ziglearn.org content. Feedback and PRs welcome.
`),
		SearchKey: []string{
			"#",
			" ",
			"ziglearn",
		},
		Sections: []schema.Section{{
			ID:         "How to run the tests",
			ShortLabel: "How to run the tests",
			Label:      schema.Markdown("How to run the tests"),
			Detail:     schema.Markdown("\n1. `zig run test-out.zig`\n2. `zig test do_tests.zig`\n"),
			SearchKey: []string{
				"#",
				" ",
				"ziglearn",
				" ",
				">",
				" ",
				"How",
				" ",
				"to",
				" ",
				"run",
				" ",
				"the",
				" ",
				"tests",
			},
			Children: []schema.Section{},
		}},
	}).Equal(t, page)
}

func Test_markdownToPage_complex(t *testing.T) {
	page := markdownToPage([]byte(`# heading1

content1

## heading2-0

content2-0

### heading3-0

content3-0

#### heading4-0

content4-0

#### heading4-1

content4-1

### heading3-1

content3-1

## heading2-1

content2-1

`), "README.md")

	autogold.Want("simple", schema.Page{
		Path: "README.md", Title: "heading1", Detail: schema.Markdown("\ncontent1\n"),
		SearchKey: []string{
			"#",
			" ",
			"heading1",
		},
		Sections: []schema.Section{
			{
				ID:         "heading2-0",
				ShortLabel: "heading2-0",
				Label:      schema.Markdown("heading2-0"),
				Detail:     schema.Markdown("\ncontent2-0\n"),
				SearchKey: []string{
					"#",
					" ",
					"heading1",
					" ",
					">",
					" ",
					"heading2-0",
				},
				Children: []schema.Section{
					{
						ID:         "heading3-0",
						ShortLabel: "heading3-0",
						Label:      schema.Markdown("heading3-0"),
						Detail:     schema.Markdown("\ncontent3-0\n"),
						SearchKey: []string{
							"#",
							" ",
							"heading1",
							" ",
							">",
							" ",
							"heading3-0",
						},
						Children: []schema.Section{
							{
								ID:         "heading4-0",
								ShortLabel: "heading4-0",
								Label:      schema.Markdown("heading4-0"),
								Detail:     schema.Markdown("\ncontent4-0\n"),
								SearchKey: []string{
									"#",
									" ",
									"heading1",
									" ",
									">",
									" ",
									"heading4-0",
								},
								Children: []schema.Section{},
							},
							{
								ID:         "heading4-1",
								ShortLabel: "heading4-1",
								Label:      schema.Markdown("heading4-1"),
								Detail:     schema.Markdown("\ncontent4-1\n"),
								SearchKey: []string{
									"#",
									" ",
									"heading1",
									" ",
									">",
									" ",
									"heading4-1",
								},
								Children: []schema.Section{},
							},
						},
					},
					{
						ID:         "heading3-1",
						ShortLabel: "heading3-1",
						Label:      schema.Markdown("heading3-1"),
						Detail:     schema.Markdown("\ncontent3-1\n"),
						SearchKey: []string{
							"#",
							" ",
							"heading1",
							" ",
							">",
							" ",
							"heading3-1",
						},
						Children: []schema.Section{},
					},
				},
			},
			{
				ID:         "heading2-1",
				ShortLabel: "heading2-1",
				Label:      schema.Markdown("heading2-1"),
				Detail:     schema.Markdown("\ncontent2-1\n\n"),
				SearchKey: []string{
					"#",
					" ",
					"heading1",
					" ",
					">",
					" ",
					"heading2-1",
				},
				Children: []schema.Section{},
			},
		},
	}).Equal(t, page)
}

func Test_markdownToPage_frontmatter(t *testing.T) {
	page := markdownToPage([]byte(`
---
title: "Mypage title"
weight: 1
date: 2021-01-23 20:52:00
description: "yay"
---

This content is not preceded by a heading.

## heading2

content2

`), "README.md")

	autogold.Want("simple", schema.Page{
		Path: "README.md", Title: "Mypage title",
		Detail: schema.Markdown(`
This content is not preceded by a heading.
`),
		SearchKey: []string{
			"#",
			" ",
			"Mypage",
			" ",
			"title",
		},
		Sections: []schema.Section{{
			ID:         "heading2",
			ShortLabel: "heading2",
			Label:      schema.Markdown("heading2"),
			Detail:     schema.Markdown("\ncontent2\n\n"),
			SearchKey: []string{
				"#",
				" ",
				"Mypage",
				" ",
				"title",
				" ",
				">",
				" ",
				"heading2",
			},
			Children: []schema.Section{},
		}},
	}).Equal(t, page)
}

func Test_markdownToPage_nonlinear_headers(t *testing.T) {
	page := markdownToPage([]byte(`# The Go Programming Language
### Download and Install
#### Binary Distributions
a
#### Install From Source
### Contributing
`), "README.md")

	autogold.Want("simple", schema.Page{
		Path: "README.md", Title: "The Go Programming Language",
		SearchKey: []string{
			"#",
			" ",
			"The",
			" ",
			"Go",
			" ",
			"Programming",
			" ",
			"Language",
		},
		Sections: []schema.Section{
			{
				ID:         "Download and Install",
				ShortLabel: "Download and Install",
				Label:      schema.Markdown("Download and Install"),
				SearchKey: []string{
					"#",
					" ",
					"The",
					" ",
					"Go",
					" ",
					"Programming",
					" ",
					"Language",
					" ",
					">",
					" ",
					"Download",
					" ",
					"and",
					" ",
					"Install",
				},
				Children: []schema.Section{
					{
						ID:         "Binary Distributions",
						ShortLabel: "Binary Distributions",
						Label:      schema.Markdown("Binary Distributions"),
						Detail:     schema.Markdown("a"),
						SearchKey: []string{
							"#",
							" ",
							"The",
							" ",
							"Go",
							" ",
							"Programming",
							" ",
							"Language",
							" ",
							">",
							" ",
							"Binary",
							" ",
							"Distributions",
						},
						Children: []schema.Section{},
					},
					{
						ID:         "Install From Source",
						ShortLabel: "Install From Source",
						Label:      schema.Markdown("Install From Source"),
						SearchKey: []string{
							"#",
							" ",
							"The",
							" ",
							"Go",
							" ",
							"Programming",
							" ",
							"Language",
							" ",
							">",
							" ",
							"Install",
							" ",
							"From",
							" ",
							"Source",
						},
						Children: []schema.Section{},
					},
				},
			},
			{
				ID:         "Contributing",
				ShortLabel: "Contributing",
				Label:      schema.Markdown("Contributing"),
				SearchKey: []string{
					"#",
					" ",
					"The",
					" ",
					"Go",
					" ",
					"Programming",
					" ",
					"Language",
					" ",
					">",
					" ",
					"Contributing",
				},
				Children: []schema.Section{},
			},
		},
	}).Equal(t, page)
}
