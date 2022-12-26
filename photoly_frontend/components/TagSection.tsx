import { AiFillTags ***REMOVED*** from "react-icons/ai";
import TagHeader from "./TagHeader";
import TagItem from "./items/TagItem";
import { useGetAllTagsQuery ***REMOVED*** from "../redux/api/tagSlice";

const TagSection = () => {
  const { data: tags, error, isLoading ***REMOVED*** = useGetAllTagsQuery();
  return (
    <>
      <TagHeader headerIcon={AiFillTags***REMOVED*** iconColor="teal.400">
        Your Tags
      </TagHeader>
  ***REMOVED***isLoading && (<p>loading...</p>)***REMOVED***
  ***REMOVED***!isLoading && tags && (tags.map((t, ind) => (
        <TagItem tagName={t.tagName***REMOVED*** tagId={t.tagId***REMOVED*** key={ind***REMOVED*** />
      )))***REMOVED***
    </>
  );
***REMOVED***;

export default TagSection;
