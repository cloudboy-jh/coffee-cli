class Coffee < Formula
  desc "Tiny macOS CLI to temporarily keep a Mac awake"
  homepage "https://github.com/cloudboy-jh/coffee-cli"
  head "https://github.com/cloudboy-jh/coffee-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-X main.version=#{version}")
  end

  test do
    ENV["HOME"] = testpath/"home"

    assert_equal "coffee #{version}\n", shell_output("#{bin}/coffee --version")
    assert_equal "☕ Resting\n", shell_output("#{bin}/coffee status")
  end
end
