import { createFileRoute, useNavigate } from '@tanstack/react-router';

import { ReaderScreen } from '@/components/reader/reader-screen';

export const Route = createFileRoute('/read/$bookId')({
  component: ReaderPage,
});

function ReaderPage() {
  const { bookId } = Route.useParams();
  const navigate = useNavigate();
  return (
    <ReaderScreen
      bookId={bookId}
      onPracticeStrokes={(char) =>
        navigate({ to: '/characters/$char', params: { char } })
      }
    />
  );
}
