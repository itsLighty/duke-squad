import styles from "./page.module.css";
import dynamic from "next/dynamic";

// Import ThemeToggle and CopyButton dynamically to prevent hydration issues
const ThemeToggle = dynamic(() => import("./components/ThemeToggle"), {
});

const CopyButton = dynamic(() => import("./components/CopyButton"), {
});

export default function Home() {
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>duke squad</h1>
        <div className={styles.headerActions}>
          <a
            className={styles.headerButton}
            href="https://github.com/itsLighty/duke-squad"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub
          </a>
          <a
            href="https://github.com/itsLighty/duke-squad#readme"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.headerButton}
          >
            Docs
          </a>
          <ThemeToggle />
        </div>
      </header>
      <main className={styles.main}>

        
        <p className={styles.tagline}>
          Manage multiple AI agents like <span className={styles.highlight}>Claude Code</span>, <span className={styles.highlight}>Codex</span>, and <span className={styles.highlight}>Gemini</span> across local folders and SSH hosts. <br/><span className={styles.tenx}>10x</span> your productivity
        </p>

        <div className={styles.demoVideo}>
          <video 
            controls
            autoPlay
            muted
            loop
            playsInline
            className={styles.video}
            src="https://github.com/user-attachments/assets/aef18253-e58f-4525-9032-f5a3d66c975a"
          />
        </div>

        <div className={styles.installation}>
          <h2>Installation</h2>
          <h3>Via Shell Script</h3>
          <div className={styles.codeBlockWrapper}>
            <pre className={styles.codeBlock}>
              <code>curl -fsSL https://raw.githubusercontent.com/itsLighty/duke-squad/main/install.sh | bash</code>
            </pre>
            <CopyButton textToCopy="curl -fsSL https://raw.githubusercontent.com/itsLighty/duke-squad/main/install.sh | bash" />
          </div>
          <br></br>
          <h3>Build Locally</h3>
          <div className={styles.codeBlockWrapper}>
            <pre className={styles.codeBlock}>
              <code>go build -o build/duke-squad .</code>
            </pre>
            <CopyButton textToCopy="go build -o build/duke-squad ." />
          </div>
          <p className={styles.prerequisites}>
            The installer adds a canonical <code>duke-squad</code> binary and a <code>ds</code> shortcut. Prerequisites: tmux, gh (GitHub CLI)
          </p>
        </div>
        
        <div className={styles.features}>
          <h2>Why use Duke Squad?</h2>
          <ul>
            <li>Supervise multiple agents in one UI</li>
            <li>Add local folders or remote projects over SSH</li>
            <li>Isolate tasks in git workspaces or managed snapshots</li>
            <li>Review work before shipping</li>
          </ul>
        </div>
      </main>
      <footer className={styles.footer}>
        <p className={styles.copyright}>
          &copy; {new Date().getFullYear()} Duke Squad. Licensed under <a href="https://github.com/itsLighty/duke-squad/blob/main/LICENSE.md" target="_blank" rel="noopener noreferrer">GNU AGPL v3.0</a>
        </p>
      </footer>
    </div>
  );
}
