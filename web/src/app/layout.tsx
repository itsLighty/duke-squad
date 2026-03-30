import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  userScalable: true,
};

export const metadata: Metadata = {
  title: "Duke Squad - Manage Local and SSH AI Workspaces",
  description: "A terminal app for running multiple AI coding agents across local folders and remote SSH projects with isolated workspaces.",
  keywords: ["duke squad", "ai", "code assistant", "terminal", "tmux", "ssh", "codex", "claude code", "gemini"],
  authors: [{ name: "itsLighty" }],
  openGraph: {
    title: "Duke Squad",
    description: "A terminal app that manages multiple AI code assistants across local and SSH workspaces",
    url: "https://github.com/itsLighty/duke-squad",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Duke Squad",
    description: "A terminal app that manages multiple AI code assistants across local and SSH workspaces",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${geistSans.variable} ${geistMono.variable}`}>
        {children}
      </body>
    </html>
  );
}
