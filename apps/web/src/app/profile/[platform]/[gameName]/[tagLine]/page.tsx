import type { Metadata } from "next";
import { ApiError, getMatchHistory, getProfile, type MatchSummary, type Profile } from "@/lib/api";

interface PageParams {
  platform: string;
  gameName: string;
  tagLine: string;
}

async function loadPageParams(params: Promise<PageParams>) {
  const { platform, gameName, tagLine } = await params;
  return {
    platform,
    gameName: decodeURIComponent(gameName),
    tagLine: decodeURIComponent(tagLine),
  };
}

export async function generateMetadata({
  params,
}: {
  params: Promise<PageParams>;
}): Promise<Metadata> {
  const { gameName, tagLine } = await loadPageParams(params);
  return { title: `${gameName}#${tagLine} — Feeder Site` };
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}:${String(rest).padStart(2, "0")}`;
}

function timeAgo(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const days = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  if (days <= 0) return "hoje";
  if (days === 1) return "ontem";
  return `há ${days} dias`;
}

function MatchRow({ match }: { match: MatchSummary }) {
  return (
    <div className={`match-row ${match.win ? "win" : "loss"}`}>
      <span className="match-champion">{match.championName}</span>
      <span className="match-kda">
        {match.kills}/{match.deaths}/{match.assists}
      </span>
      <span className="tag">{formatDuration(match.gameDuration)}</span>
      <span className="tag">{timeAgo(match.gameCreation)}</span>
    </div>
  );
}

export default async function ProfilePage({ params }: { params: Promise<PageParams> }) {
  const { platform, gameName, tagLine } = await loadPageParams(params);

  let profile: Profile;
  try {
    profile = await getProfile(platform, gameName, tagLine);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return (
        <div className="empty-state">
          Jogador <strong>{gameName}#{tagLine}</strong> não encontrado em {platform.toUpperCase()}.
        </div>
      );
    }
    return (
      <div className="error-state">
        Não foi possível carregar o perfil agora. Tente novamente em instantes.
      </div>
    );
  }

  let matches: MatchSummary[] = [];
  try {
    const history = await getMatchHistory(profile.account.puuid);
    matches = history.matches;
  } catch {
    // Match history is best-effort: ingestion may not have caught up yet.
  }

  const soloQueue = profile.leagues.find((l) => l.queueType === "RANKED_SOLO_5x5");
  const flexQueue = profile.leagues.find((l) => l.queueType === "RANKED_FLEX_SR");

  return (
    <div>
      <div className="profile-header">
        <h1>{profile.account.gameName}</h1>
        <span className="tag">#{profile.account.tagLine}</span>
        <span className="pill">Nível {profile.summoner.summonerLevel}</span>
        <span className="pill">{platform.toUpperCase()}</span>
      </div>

      <div className="league-grid">
        <RankedCard title="Ranked Solo/Duo" entry={soloQueue} />
        <RankedCard title="Ranked Flex" entry={flexQueue} />
      </div>

      <h2>Partidas recentes</h2>
      {matches.length === 0 ? (
        <p className="empty-state">
          Ainda processando o histórico dessa conta em segundo plano — atualize a página em
          alguns instantes.
        </p>
      ) : (
        <div>
          {matches.map((m) => (
            <MatchRow key={m.matchId} match={m} />
          ))}
        </div>
      )}
    </div>
  );
}

function RankedCard({
  title,
  entry,
}: {
  title: string;
  entry?: Profile["leagues"][number];
}) {
  if (!entry) {
    return (
      <div className="card">
        <div className="tag">{title}</div>
        <div className="empty-state">Sem elo nesta fila</div>
      </div>
    );
  }

  const totalGames = entry.wins + entry.losses;
  const winRate = totalGames > 0 ? Math.round((entry.wins / totalGames) * 100) : 0;

  return (
    <div className="card">
      <div className="tag">{title}</div>
      <div style={{ fontSize: 20, fontWeight: 700, margin: "6px 0" }}>
        {entry.tier} {entry.rank} · {entry.leaguePoints} LP
      </div>
      <div className="tag">
        {entry.wins}V / {entry.losses}D ({winRate}%)
      </div>
    </div>
  );
}
