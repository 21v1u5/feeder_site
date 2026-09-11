import Link from "next/link";
import { getTierList, RANKED_QUEUES } from "@/lib/api";

export const metadata = { title: "Tier List — Feeder Site" };

function winRateClass(winRate: number): string {
  if (winRate >= 52) return "win-rate-high";
  if (winRate < 48) return "win-rate-low";
  return "";
}

export default async function TierListPage({
  searchParams,
}: {
  searchParams: Promise<{ queueId?: string }>;
}) {
  const { queueId: queueIdParam } = await searchParams;
  const queueId = Number(queueIdParam ?? RANKED_QUEUES[0].id);

  let patch = "";
  let champions: Awaited<ReturnType<typeof getTierList>>["champions"] = [];
  let errored = false;

  try {
    const result = await getTierList(queueId);
    patch = result.patch;
    champions = result.champions;
  } catch {
    errored = true;
  }

  return (
    <div>
      <h1>Tier List</h1>
      <div className="tier-list-nav">
        {RANKED_QUEUES.map((q) => (
          <Link
            key={q.id}
            href={`/tier-list?queueId=${q.id}`}
            className={q.id === queueId ? "active" : ""}
          >
            {q.label}
          </Link>
        ))}
      </div>

      {errored ? (
        <p className="error-state">Não foi possível carregar a tier list agora.</p>
      ) : champions.length === 0 ? (
        <p className="empty-state">
          Ainda não há dados suficientes para essa fila{patch ? ` no patch ${patch}` : ""}. A
          tier list é consolidada periodicamente conforme partidas são ingeridas.
        </p>
      ) : (
        <>
          <p className="tag">Patch {patch}</p>
          <table>
            <thead>
              <tr>
                <th>#</th>
                <th>Campeão</th>
                <th>Win Rate</th>
                <th>Jogos</th>
                <th>K / D / A médios</th>
              </tr>
            </thead>
            <tbody>
              {champions.map((c, i) => (
                <tr key={c.champion}>
                  <td>{i + 1}</td>
                  <td>{c.champion}</td>
                  <td className={winRateClass(c.winRate)}>{c.winRate.toFixed(2)}%</td>
                  <td>{c.games}</td>
                  <td>
                    {c.avgKills} / {c.avgDeaths} / {c.avgAssists}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
}
