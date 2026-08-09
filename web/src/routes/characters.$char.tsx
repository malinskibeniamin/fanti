import { createFileRoute } from '@tanstack/react-router';

import { CharacterPage } from '@/components/character/character-page';

export const Route = createFileRoute('/characters/$char')({
  component: CharacterRoute,
});

function CharacterRoute() {
  const { char } = Route.useParams();
  return <CharacterPage char={char} />;
}
