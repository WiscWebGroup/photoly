import { useGetChildrenQuery, useGetRootQuery ***REMOVED*** from "../redux/api/namespaceSlice";
import { skipToken ***REMOVED*** from "@reduxjs/toolkit/query";
import { useGetPhotoByAlbumQuery, useGetPhotoByNamespaceQuery, useGetPhotoByTagsQuery ***REMOVED*** from "../redux/api/photoSlice";
import { useAppSelector ***REMOVED*** from "../redux/hooks";
import { ISearchFilter ***REMOVED*** from "../redux/states/searchFilterSlice";

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

function isFilterClear(filter: ISearchFilter) {
  return filter.albumId === -1 && filter.tagIds.length === 0;
***REMOVED***
function isTagApply(filter: ISearchFilter) {
  return filter.tagIds.length > 0;
***REMOVED***
function isAlbumApply(filter: ISearchFilter) {
  return filter.albumId !== -1
***REMOVED***

export default function useGlobalSearch(): ISearchData {
  const curFilter = useAppSelector((state) => state.searchFilter);

  // If no filter apply, then query the root folder
  const {
    data: root_dir,
    isSuccess: isRootSuccess
  ***REMOVED*** = useGetRootQuery(isFilterClear(curFilter) ? undefined : skipToken);
  const {
    data: children_dirs,
    isSuccess: isChildrenSuccess
  ***REMOVED*** = useGetChildrenQuery(isRootSuccess ? root_dir.nsId : skipToken);
  const {
    data: photos,
    isSuccess: isPhotoSuccess
  ***REMOVED*** = useGetPhotoByNamespaceQuery(isRootSuccess ? root_dir.nsId : skipToken);

  // If tag filter apply
  const {
    data: tag_photos,
    isSuccess: isTagPhotoSuccess
  ***REMOVED*** = useGetPhotoByTagsQuery(isTagApply(curFilter) ? curFilter.tagIds : skipToken)

  // If album filter apply
  const {
    data: album_photos,
    isSuccess: isAlbumPhotoSuccess
  ***REMOVED*** = useGetPhotoByAlbumQuery(isAlbumApply(curFilter) ? curFilter.albumId: skipToken)

  if (isFilterClear(curFilter) && isChildrenSuccess && isPhotoSuccess && isRootSuccess) {
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

    let rootFolder: IFolder = {
      id: root_dir.nsId, name: root_dir.nsName, parentId: root_dir.nsParentId, userId: root_dir.userId
***REMOVED***;

    return {
      path: [rootFolder],
      current: rootFolder,
      photos: photoData,
      folders: folderData
***REMOVED***;
  ***REMOVED***

  if (isTagApply(curFilter) && isTagPhotoSuccess) {
    let photoData: IPhoto[] = [];
    tag_photos.forEach((e) => {
      photoData.push({
        id: e.photoId,
        name: e.photoName,
        format: e.format,
        uploaddate: e.uploadDate,
        visibility: !!e.visibility
  ***REMOVED***);
***REMOVED***);
    return {
      path: [defaultFolder],
      current: defaultFolder,
      photos: photoData,
      folders: []
***REMOVED***;
  ***REMOVED***

  if (isAlbumApply(curFilter) && isAlbumPhotoSuccess) {
    let photoData: IPhoto[] = [];
    album_photos.forEach((e) => {
      photoData.push({
        id: e.photoId,
        name: e.photoName,
        format: e.format,
        uploaddate: e.uploadDate,
        visibility: !!e.visibility
  ***REMOVED***);
***REMOVED***);
    return {
      path: [defaultFolder],
      current: defaultFolder,
      photos: photoData,
      folders: []
***REMOVED***;
  ***REMOVED***

  return initSearchDataState;
***REMOVED***