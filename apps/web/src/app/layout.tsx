import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "Feeder Site — Perfis e Tier List de League of Legends",
  description:
    "Consulte perfis, histórico de partidas e tier lists de League of Legends.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="pt-BR">
      <body>
        <nav className="nav">
          <Link href="/" className="brand">
            Feeder Site
          </Link>
          <Link href="/">Buscar Perfil</Link>
          <Link href="/tier-list">Tier List</Link>
        </nav>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
