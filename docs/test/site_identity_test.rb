require "minitest/autorun"

class SiteIdentityTest < Minitest::Test
  DOCS_ROOT = File.expand_path("..", __dir__)
  REPO_ROOT = File.expand_path("../..", __dir__)
  PUBLISHED_FILES = Dir[
    File.join(DOCS_ROOT, "**", "*.{html,md,yml,yaml}"),
    File.join(REPO_ROOT, "README.md")
  ].reject { |path| path.include?("/_site/") || path.include?("/test/") }

  def combined_content
    PUBLISHED_FILES.map { |path| File.read(path) }.join("\n")
  end

  def test_primary_domain_is_gptcode_dev
    assert_equal "gptcode.dev", File.read(File.join(DOCS_ROOT, "CNAME")).strip
    assert_includes File.read(File.join(DOCS_ROOT, "_config.yml")), 'url: "https://gptcode.dev"'
  end

  def test_site_points_to_personal_repository
    assert_includes combined_content, "https://github.com/jadercorrea/gptcode"
    refute_includes combined_content, "github.com/gptcode-cloud"
  end

  def test_retired_products_are_not_published
    refute_match(/gptcode[ .-]?(live|cloud)/i, combined_content)
    refute_match(%r{https?://gptcode\.(live|cloud)}i, combined_content)
    refute_path_exists File.join(DOCS_ROOT, "live.html")
    refute_path_exists File.join(DOCS_ROOT, "pricing.html")
  end

  def test_homepage_presents_an_open_source_project
    homepage = File.read(File.join(DOCS_ROOT, "index.md"))

    assert_match(/open source/i, homepage)
    assert_includes homepage, "Jader Correa"
    assert_includes homepage, "go install github.com/jadercorrea/gptcode"
    assert_includes homepage, "AI coding agents should produce"
    assert_includes homepage, "evidence, not just answers."
    assert_includes homepage, "Investigate"
    assert_includes homepage, "Executable verification"
    assert_includes homepage, "Why I built GPTCode"
    assert_includes homepage, "<span>Models</span> generate possibilities."
    assert_includes homepage, "<span>Repositories</span> define constraints."
    assert_includes homepage, "<span>Verification</span> establishes truth."
    assert_includes homepage, "Repository-centered architecture"
    assert_includes homepage, "Inspect the evidence"
    assert_includes homepage, "Reduce onboarding time"
    assert_includes homepage, "Reduce implementation risk"
    assert_includes homepage, "Reliable AI systems are built on explicit constraints, not optimistic prompts."
    assert_includes homepage, "One model rarely excels at every task."
    assert_includes homepage, "GPTCode decomposes software development into explicit stages"
    assert_includes homepage, "Race detector · 100% statement coverage"
    refute_match(/not presented as|not a hosted/i, homepage)
    refute_includes homepage, "One tool, multiple models"
  end

  def test_navigation_exposes_product_thesis
    layout = File.read(File.join(DOCS_ROOT, "_layouts", "default.html"))

    %w[Overview Workflows Skills Architecture Docs Blog GitHub].each do |label|
      assert_includes layout, ">#{label}<"
    end
  end

  def test_test_suite_is_not_published
    assert_includes File.read(File.join(DOCS_ROOT, "_config.yml")), "  - test"
  end

  def test_site_exposes_search_and_answer_engine_metadata
    layout = File.read(File.join(DOCS_ROOT, "_layouts", "default.html"))
    config = File.read(File.join(DOCS_ROOT, "_config.yml"))

    assert_includes layout, "{% seo %}"
    assert_includes config, "jekyll-sitemap"
    assert_includes config, "https://www.linkedin.com/in/jadercorrea"
    assert_path_exists File.join(DOCS_ROOT, "robots.txt")
    assert_path_exists File.join(DOCS_ROOT, "llms.txt")
  end

  def test_footer_connects_the_project_to_its_creator
    layout = File.read(File.join(DOCS_ROOT, "_layouts", "default.html"))

    assert_includes layout, "Created by Jader Correa"
    assert_includes layout, "Principal engineer building AI agents"
    assert_includes layout, "https://jader-correa.com"
  end

  def test_homepage_embeds_a_real_terminal_recording
    homepage = File.read(File.join(DOCS_ROOT, "index.md"))
    recording = File.read(File.join(DOCS_ROOT, "assets", "gptcode-workflow.cast"))

    assert_includes homepage, "/assets/gptcode-workflow.gif"
    assert_includes homepage, "/assets/gptcode-workflow.mp4"
    assert_includes homepage, "controls"
    assert_path_exists File.join(DOCS_ROOT, "assets", "gptcode-workflow.gif")
    assert_path_exists File.join(DOCS_ROOT, "assets", "gptcode-workflow.mp4")
    assert_path_exists File.join(DOCS_ROOT, "assets", "gptcode-workflow.cast")
    assert_includes recording, "TestStoreSupportsConcurrentAccess"
    assert_includes recording, "go test -race ./..."
    assert_includes recording, "Verification passed"
    assert_includes recording, "evidence-driven repair"
    assert_includes recording, "public API"
    assert_includes recording, '"x", "0"'
    refute_includes recording, "package errors is not in std"
    refute_includes homepage, "repository detection and skills"
  end

  def test_public_example_is_executable_evidence
    homepage = File.read(File.join(DOCS_ROOT, "index.md"))
    workflow = File.read(File.join(REPO_ROOT, ".github", "workflows", "ci.yml"))
    verifier = File.read(File.join(REPO_ROOT, "scripts", "verify-public-examples.sh"))

    assert_path_exists File.join(REPO_ROOT, "examples", "sessionstore", "store.go")
    assert_path_exists File.join(REPO_ROOT, "examples", "sessionstore", "store_test.go")
    assert_includes homepage, "examples/sessionstore"
    assert_includes workflow, "scripts/verify-public-examples.sh"
    assert_includes verifier, "go test -race"
    assert_includes verifier, 'coverage" != "100.0"'
  end

  def test_releases_require_explicit_tags_and_quality_gates
    workflow = File.read(File.join(REPO_ROOT, ".github", "workflows", "cd.yml"))
    goreleaser = File.read(File.join(REPO_ROOT, ".goreleaser.yml"))

    assert_includes workflow, 'tags:'
    assert_includes workflow, 'go test -v -short ./...'
    assert_includes workflow, 'scripts/verify-public-examples.sh'
    assert_includes workflow, 'args: release --clean'
    refute_includes workflow, 'schedule:'
    refute_includes workflow, 'gptcode-cloud'
    refute_includes goreleaser, 'disable: true'
  end

  def test_anchor_essay_is_published_and_evidence_driven
    article = File.join(
      DOCS_ROOT,
      "_posts",
      "2026-07-25-the-workflow-is-the-source-of-truth.md"
    )

    assert_path_exists article

    content = File.read(article)
    assert_includes content, "The Workflow Is the Source of Truth"
    assert_includes content, "Models generate possibilities."
    assert_includes content, "Repositories define constraints."
    assert_includes content, "Verification establishes truth."
    assert_includes content, "The first experiment failed"
    assert_includes content, "likely Python"
    assert_includes content, "fail fast"
    assert_includes content, "Research → Plan → Implement → Review → Verify"
    assert_includes content, "Turning the failure into an executable contract"
    assert_includes content, "scripts/verify-public-examples.sh"
    assert_includes content, "100%"
    assert_includes content, "gptcode.dev"

    blog = File.read(File.join(DOCS_ROOT, "blog.html"))
    assert_includes blog, "Essays on reliable AI agents"
  end
end
