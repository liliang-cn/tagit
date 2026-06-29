class Tagit < Formula
  desc "Daemon-first orchestrator for coding-agent CLIs (claude, codex, ...)"
  homepage "https://github.com/liliang-cn/tagit"
  head "https://github.com/liliang-cn/tagit.git", branch: "main"
  license "MIT"
  depends_on "go" => :build

  def install
    ENV["GOWORK"] = "off"
    system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"tagit"), "./cmd/tagit"
    system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"tagitd"), "./cmd/tagitd"
  end

  test do
    assert_match "tagit usage", shell_output("#{bin}/tagit --help")
  end
end
