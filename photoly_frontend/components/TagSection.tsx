import { AiFillTags } from "react-icons/ai";
import TagHeader from "./TagHeader";
import TagItem from "./items/TagItem";
import { useGetAllTagsQuery } from "../redux/api/tagSlice";

const TagSection = () => {
  const { data: tags, error, isLoading } = useGetAllTagsQuery();
  return (
    <>
      <TagHeader headerIcon={AiFillTags} iconColor="teal.400">
        Your Tags
      </TagHeader>
      {isLoading && (<p>loading...</p>)}
      {!isLoading && tags && (tags.map((t, ind) => (
        <TagItem tagName={t.tagName} tagId={t.tagId} key={ind} />
      )))}
    </>
  );
};

export default TagSection;
