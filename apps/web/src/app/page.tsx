import { PLATFORMS } from "@/lib/api";

export default function HomePage() {
  return (
    <div>
      <h1>Buscar perfil</h1>
      <p className="tag">
        Informe a plataforma e o Riot ID (nome e tag, sem o &quot;#&quot;).
      </p>
      <form action="/search" method="GET" className="search-form card">
        <select name="platform" defaultValue="na1">
          {PLATFORMS.map((p) => (
            <option key={p.id} value={p.id}>
              {p.label}
            </option>
          ))}
        </select>
        <input name="gameName" placeholder="Nome (ex: Faker)" required />
        <input name="tagLine" placeholder="TAG" required />
        <button type="submit">Buscar</button>
      </form>
    </div>
  );
}
