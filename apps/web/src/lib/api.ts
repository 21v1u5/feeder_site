// Server-side client for the Go backend. Every call here runs on the
// Next.js server (SSR / server components), never in the browser, so the
// backend never needs to be reachable from the client's network and no
// CORS setup is required.
const API_BASE_URL = process.env.API_BASE_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, init);
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new ApiError(res.status, body || res.statusText);
  }
  return res.json() as Promise<T>;
}

export interface Account {
  puuid: string;
  gameName: string;
  tagLine: string;
}

export interface Summoner {
  id: string;
  accountId: string;
  puuid: string;
  profileIconId: number;
  revisionDate: number;
  summonerLevel: number;
}

export interface LeagueEntry {
  leagueId: string;
  queueType: string;
  tier: string;
  rank: string;
  summonerId: string;
  leaguePoints: number;
  wins: number;
  losses: number;
}

export interface Profile {
  account: Account;
  summoner: Summoner;
  leagues: LeagueEntry[];
  recentMatchIds: string[];
}

export interface MatchSummary {
  matchId: string;
  platform: string;
  gameCreation: string;
  gameDuration: number;
  gameVersion: string;
  queueId: number;
  championName: string;
  win: boolean;
  kills: number;
  deaths: number;
  assists: number;
}

export interface ChampionStat {
  patch: string;
  queueId: number;
  champion: string;
  games: number;
  wins: number;
  winRate: number;
  avgKills: number;
  avgDeaths: number;
  avgAssists: number;
}

export function getProfile(platform: string, gameName: string, tagLine: string) {
  const path = `/api/profiles/${encodeURIComponent(platform)}/${encodeURIComponent(gameName)}/${encodeURIComponent(tagLine)}`;
  return apiFetch<Profile>(path, { cache: "no-store" });
}

export function getMatchHistory(puuid: string, limit = 20) {
  return apiFetch<{ matches: MatchSummary[] }>(
    `/api/accounts/${encodeURIComponent(puuid)}/matches?limit=${limit}`,
    { cache: "no-store" },
  );
}

export function getTierList(queueId: number, patch?: string, minGames = 10) {
  const params = new URLSearchParams({ queueId: String(queueId), minGames: String(minGames) });
  if (patch) params.set("patch", patch);
  // The tier list is a materialized view refreshed on a schedule (see the
  // backend's TIER_LIST_REFRESH_PERIOD), so a short revalidate window here
  // is just caching a cache - safe to keep for a minute at a time.
  return apiFetch<{ patch: string; champions: ChampionStat[] }>(`/api/tier-list?${params}`, {
    next: { revalidate: 60 },
  });
}

export const RANKED_QUEUES = [
  { id: 420, label: "Ranked Solo/Duo" },
  { id: 440, label: "Ranked Flex" },
];

export const PLATFORMS = [
  { id: "na1", label: "NA" },
  { id: "br1", label: "BR" },
  { id: "la1", label: "LAN" },
  { id: "la2", label: "LAS" },
  { id: "euw1", label: "EUW" },
  { id: "eun1", label: "EUNE" },
  { id: "tr1", label: "TR" },
  { id: "ru", label: "RU" },
  { id: "kr", label: "KR" },
  { id: "jp1", label: "JP" },
];
