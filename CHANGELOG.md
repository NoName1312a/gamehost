# Changelog

All notable changes to GameNest are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/).

`scripts/release.ps1` reads the section matching the version you publish and uses
it as the in-app update notes **and** the GitHub release body. Before releasing,
rename `[Unreleased]` to the version you're shipping (e.g. `## [0.4.0] - 2026-06-10`).

## [Unreleased]

## [0.6.11] - 2026-07-31

### Fixed
- **Veloren servers could not be created at all.** GameNest pointed at a community server image that stopped publishing the version it asked for, so picking Veloren failed while every other game worked. It now uses the Veloren project's own dedicated server, pinned to the stable release your game client installs — and its save data and the query port that server browsers use are configured correctly, which they were not before.

## [0.6.10] - 2026-07-31

### Fixed
- **Friend requests could reach the wrong person.** Searching for a username containing an underscore also matched similar names — looking for `leo_player` could find `leoXplayer`. Usernames are now matched exactly.
- Checking whether a username is free had the same flaw and could report a free name as taken.

### Security
- The file listing your servers is no longer world-readable. It holds server passwords, so it is now readable only by your account.
- Released builds no longer trust the development server's address. Nothing could reach it in practice, but a shipped app has no business trusting it.

## [0.6.9] - 2026-07-30

### Fixed
- **Changing a server's port broke the "friends can join" address without saying so.** The tunnel kept pointing at the old port, so the address GameNest showed still looked fine but nobody could connect. Editing a port now updates the tunnel properly — and adding or removing a port gets a fresh address instead of a stale one.
- **"New server" did nothing if the game list failed to load.** Clicking it now explains what went wrong, says your existing servers are unaffected, and offers to retry.
- **Turning on remote access could break signing in with Discord.** Both were using the same port. Remote access now suggests a different one; if you already picked that port yourself, it stays as you set it.
- **An abandoned sign-in blocked the next one.** Closing the browser tab midway used to hold the sign-in port until you restarted GameNest. It now frees itself.

### Changed
- The "GameNest isn't responding" screen now explains what to do — it used to print developer commands that only make sense if you built the app from source.

## [0.6.8] - 2026-07-29

### Security
- **Shared accounts can no longer touch the whole machine.** If you turn on remote access and hand an admin or operator account to a friend, they can run your game servers — but they can no longer wipe the install, switch remote access on or off, point off-site backups anywhere they like, or change which GameNest account this PC belongs to. Those stay with the owner.
- **The app window is locked down.** GameNest now restricts what its own interface is allowed to load and where it may send data — to your local engine and your GameNest account, and nowhere else. It's a seatbelt: if a third-party package the app depends on were ever compromised, it can't quietly phone home.

## [0.6.7] - 2026-07-28

### Security
- **A malicious website could take over your servers just by being visited.** The engine trusted any request that reached it locally, so a web page could quietly reach in and delete servers, read server files, or change settings behind your back. The engine now verifies that requests really come from the GameNest app and rejects the rest.

### Changed
- **Signing in is now account-based.** Create your GameNest account on gamenest.cc (username, email, password — you'll get a confirmation mail), then sign in here with it. Discord stays as the one-click alternative. The app no longer creates accounts itself; "Create one on the web" and "Forgot password?" open the website.
- Names like `Admin` or `Support` are properly reserved now, whatever the capitalisation.

## [0.6.6] - 2026-07-05

### Fixed
- **"Add server" listed no games on installed builds.** The app couldn't find its bundled game library (it looked in the wrong folder, and releases built in CI lost the fallback that masked this on the dev machine). The engine now also finds templates sitting right next to it, so the picker reliably shows all 28 games.
- The game picker now says when the library is empty instead of showing "No games match".

## [0.6.5] - 2026-06-29

### Added
- **Accounts (optional).** Sign in with Discord or email to unlock the new social features. Hosting and "friends can join" still work with no account at all.
- **Friends.** Add friends by username, then see who's online and what they're hosting — right from the sidebar.
- **Levels.** Earn XP for hosting servers and connecting with friends, and level up your profile.

## [0.6.1] - 2026-06-28

### Fixed
- **"Friends can join" now works out of the box.** The relay tunnel runs alongside your servers instead of as a separate program that Windows often blocked — no setup, no pasting addresses, it just works.
- **Modpack servers no longer crash-loop or run out of memory.** A server that fails to start now stops cleanly instead of restarting forever, and servers get extra memory headroom so large modpacks aren't killed mid-load.
- **You can always delete a server** — even one that's stuck or stopped responding.

## [0.6.0] - 2026-06-26

### Changed
- **"Friends can join" now runs on GameNest's own relay.** When your router lets the connection through, friends connect to you **directly** (lowest latency). When it doesn't, GameNest **automatically** routes through its own relay so they can still join — no setup, no second app, nothing to paste back.

### Removed
- The old **playit.gg** fallback is gone — sharing is fully built into GameNest now.

## [0.5.0] - 2026-06-25

### Added
- **Guided first-run onboarding.** New users get a Welcome → quick Docker setup → pick-a-game → "You're live" flow that walks you all the way to a running server, with the share address ready to send to a friend.
- **"Get started" checklist** on the Dashboard — Set up Docker · Create your first server · Invite a friend — that tracks real progress and auto-hides once you're set up.

### Changed
- The Docker setup screen now matches the rest of the app (glass panels, brand fonts), completing the app-wide redesign.

## [0.4.9] - 2026-06-24

### Changed
- **Redesigned the server view.** Each server now opens as a tabbed page right inside the main window — **Overview · Console · Files · Settings · Backups** (plus **Mods** for Minecraft) — instead of separate full-screen console and file windows.
- **Settings** and **Account** now open in the main window too, alongside the sidebar, rather than as pop-up dialogs.
- A visual refresh across the new-server, sign-in, engine-offline, and "What's New" screens to match the GameNest look.

## [0.4.8] - 2026-06-24

### Added
- A dedicated **Account** screen (sidebar → Account) for GameNest Plus sign-in and status, separate from Settings.

## [0.4.7] - 2026-06-24

### Changed
- **Refreshed look.** The app now shares the GameNest website's design — its fonts, the hexagon logo throughout, a subtle background glow, and frosted-glass cards.
- **New navigation.** A persistent left sidebar (Dashboard, your servers with live status, "+ New server", and quick links) replaces the old hamburger menu.
- The desktop app is now fully branded **GameNest**, with a matching app icon.

## [0.4.6] - 2026-06-14

### Added
- Configurable settings (server name, password, admin password, max players) in the create-server form for **Mordhau**, **Conan Exiles**, **Squad**, and **Insurgency: Sandstorm**.

### Fixed
- The live console no longer leaks a background log-streaming process when you close it — its lifetime is now bounded and cleaned up reliably.

### Changed
- Games configured via files (Veloren, OpenTTD, Vintage Story, Unturned) now say so in their description and point you to the file manager, instead of looking like they have no settings.

## [0.4.5] - 2026-06-14

### Changed
- **Friendlier error messages.** Raw Docker/engine errors are now shown as short, plain-language messages with a next step — e.g. "Docker isn't running — open Docker Desktop", "That port is already in use — pick another", "Not enough memory — lower the server's memory."
- **Smarter "friends can connect" flow.** After your router forwards a port, GameNest now automatically checks whether friends can actually reach it (no need to click "Test"). If the router didn't forward it — or forwarded it but an ISP/firewall is still blocking it (e.g. CGNAT) — the relay option is surfaced automatically with a plain-language explanation, instead of leaving you to figure it out.

## [0.4.4] - 2026-06-14

### Security
- Closed a cross-site request forgery (CSRF) hole. The engine trusts requests from your own machine, so a website you opened in a browser while GameNest was running could quietly send commands to it — including destructive ones like wiping all servers. Every state-changing request now requires a first-party header that other websites cannot send.

## [0.4.3] - 2026-06-14

### Added
- A **menu drawer** (the ☰ button, top-left) that gathers navigation in one place: Settings, What's New, and GitHub / Discord links.
- A **What's New** viewer — browse the full version history with what changed in each release, any time, from the menu.
- After an app update, a **What's New popup** automatically highlights what changed since the version you were last on.

## [0.4.2] - 2026-06-14

### Changed
- Reworked **Add a server**: the full-page wall of game cover art is gone. A **"+ New server"** button now opens a fast, searchable game picker (type to filter, arrow keys + Enter to pick). The dashboard is much shorter and focused on your servers.

## [0.4.1] - 2026-06-11

### Fixed
- The Settings dialog now scrolls within itself on shorter windows, instead of scrolling the dashboard behind it. The background no longer moves while Settings is open.

## [0.4.0] - 2026-06-11

### Changed
- **GameNest is now free and open source (AGPLv3).** Every feature is unlocked for everyone — there is no longer a paid "Pro" tier. Unlimited concurrent servers, scheduled backups & restarts, off-site backups, and the Minecraft mod manager are now free for all.
- Renamed the app from "GameHost" to **GameNest** (the engine, repo, and updater feed keep the internal `gamehost` name for now).

### Removed
- The Pro paywall and all license-key feature gating. (A managed hosted version and optional supporter keys are planned; the desktop app stays free forever. The Settings "Plan" section is now "Supporter / Hosted".)

### Added
- Opt-in, off-by-default **diagnostics**: anonymous crash and basic usage reports, with a Settings toggle. No personal data is collected, and nothing is sent unless you opt in.
- Settings **"Danger zone -> Remove all servers"** to delete every server, world, and Docker volume in one step.
- The **uninstaller now offers** (opt-in) to remove all game data (Docker containers, volumes, and the data folder) so a clean uninstall leaves nothing behind.
- Open-source project files: `LICENSE` (AGPL-3.0), `CONTRIBUTING.md`, `SECURITY.md`, `NOTICE`, and an `ee/` directory reserved for future commercial features.

## [0.3.0] - 2026-06-07

### Added
- Cover-art game library with grouped cards, a modernized dashboard, and first-run image-download progress.
- File manager, manual + scheduled backups, daily restart schedules, and 25+ game templates.
- Operator accounts with roles, remote (LAN/HTTPS) access, off-site backups, the Minecraft mod manager, and offline Pro licensing.
