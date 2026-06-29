class Tagit < Formula
  desc "Daemon-first orchestrator for coding-agent CLIs (claude, codex, ...)"
  homepage "https://github.com/liliang-cn/tagit"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_arm64.tar.gz"
      sha256 "f7663feb2d5f2f713019b838acb22b5c22b1f2adf6fc406713c9fc8742f97f41"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_amd64.tar.gz"
      sha256 "0db8b7a0776d3f3789ad0bbb991a83ccbaefe4a92f00f27761c1f6be60de4cee"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_arm64.tar.gz"
      sha256 "9f7322391812808b4ee9803504e9a3f48595fb5f4965af92cfe2be0db536441f"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_amd64.tar.gz"
      sha256 "1509bb691ceca6a198a295df9252f93e1a786d1036c46dda67663d070c094315"
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
