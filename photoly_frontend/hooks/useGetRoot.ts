import { useGetChildrenQuery, useGetRootQuery ***REMOVED*** from "../redux/api/namespaceSlice";
import { skipToken ***REMOVED*** from "@reduxjs/toolkit/query";
import { useGetPhotoByNamespaceQuery ***REMOVED*** from "../redux/api/photoSlice";

interface IPhoto {
  id: number;
  name: string;
  format: string;
  uploaddate: string;
  visibility: boolean; // why is this boolean?
***REMOVED***

interface IFolder {
  id: number;
  name: string;
  parentId: number;
  userId: number;
***REMOVED***

interface ISearchData {
  path: IFolder[];
  current: IFolder;
  folders: IFolder[];
  photos: IPhoto[];
***REMOVED***

const defaultFolder: IFolder = {
  id: -1,
  name: "/",
  parentId: -1,
  userId: -1
***REMOVED***;

const initSearchDataState: ISearchData = {
  path: [defaultFolder],
  current: defaultFolder,
  folders: [],
  photos: []
***REMOVED***;

export default function useGetRoot(): ISearchData {
  const { data: root_dir, isSuccess: isRootSuccess ***REMOVED*** = useGetRootQuery();
  const {
    data: children_dirs,
    isSuccess: isChildrenSuccess
  ***REMOVED*** = useGetChildrenQuery(isRootSuccess ? root_dir.nsId : skipToken);
  const {
    data: photos,
    isSuccess: isPhotoSuccess
  ***REMOVED*** = useGetPhotoByNamespaceQuery(isRootSuccess ? root_dir.nsId : skipToken);

  if (isChildrenSuccess && isPhotoSuccess && isRootSuccess) {
    let photoData: IPhoto[] = [];
    photos.forEach((e) => {
      photoData.push({
        id: e.photoId,
        name: e.photoName,
        format: e.format,
        uploaddate: e.uploadDate,
        visibility: !!e.visibility
  ***REMOVED***);
***REMOVED***);

    let folderData: IFolder[] = [];
    children_dirs.forEach((e) => {
      folderData.push({
        id: e.nsId,
        name: e.nsName,
        parentId: e.nsParentId,
        userId: e.userId
  ***REMOVED***);
***REMOVED***);

    return {
      path: [{ id: root_dir.nsId, name: root_dir.nsName, parentId: root_dir.nsParentId, userId: root_dir.userId ***REMOVED***],
      current: { id: root_dir.nsId, name: root_dir.nsName, parentId: root_dir.nsParentId, userId: root_dir.userId ***REMOVED***,
      photos: photoData,
      folders: folderData
***REMOVED***;
  ***REMOVED***

  return initSearchDataState;
***REMOVED***