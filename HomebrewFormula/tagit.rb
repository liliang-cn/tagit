class Tagit < Formula
  desc "Daemon-first orchestrator for coding-agent CLIs (claude, codex, ...)"
  homepage "https://github.com/liliang-cn/tagit"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_arm64.tar.gz"
      sha256 "b3c62c2e87e75a84cdccf03039456af32c6bcdbdf1fbca34800dc15e2e8909ad"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_amd64.tar.gz"
      sha256 "e9d844c6ba2bd57803de7f8406ff1f41a50f377d31999b7ec6be92acd4387293"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_arm64.tar.gz"
      sha256 "c33c1c051b059501d1c59bfa8fdd1a269dd5a3e03b9f6c071e05c147b1290d87"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_amd64.tar.gz"
      sha256 "905de8c5f98c43b4068de1da46583d6d4b0256ab824cd76316842971f4f96ba1"
    end
  end

  # Prebuilt binaries — no Go toolchain required.
  def install
    bin.install "tagit", "tagitd"
  end

  # `brew services start tagit` runs the daemon on login and keeps it alive.
  # The daemon uses ~/.tagit for state and config (agents.json, feishu.json, slack.json).
  service do
    run [opt_bin/"tagitd"]
    keep_alive true
    log_path var/"log/tagit.log"
    error_log_path var/"log/tagit.log"
  end

  test do
    assert_match "tagit usage", shell_output("#{bin}/tagit --help")
  end
end
