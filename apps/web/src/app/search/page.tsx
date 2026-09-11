import { redirect } from "next/navigation";

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ platform?: string; gameName?: string; tagLine?: string }>;
}) {
  const { platform, gameName, tagLine } = await searchParams;

  if (!platform || !gameName || !tagLine) {
    redirect("/");
  }

  redirect(
    `/profile/${encodeURIComponent(platform)}/${encodeURIComponent(gameName)}/${encodeURIComponent(tagLine)}`,
  );
}
