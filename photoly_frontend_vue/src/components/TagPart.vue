<script setup>
import axios from "axios";
import {Archive16Regular as ArchiveIcon, FolderAdd20Regular, FolderArrowRight20Regular, ArrowUpload20Filled,
  ArrowDownload20Filled, Eye20Regular, Delete20Regular, Edit20Regular } from "@vicons/fluent"
import { ref, defineComponent, nextTick, h } from 'vue';
import { NTag } from "naive-ui";
import { useMessage, NIcon } from 'naive-ui'
import MoveToShowFolder from "./MoveToShowFolder.vue"
import MediaList from '@/components/MediaList.vue'
</script>

<template>
  <div>

    <!-- This part is for medias -->

    <div @contextmenu="blankSpaceMenu">

      <!-- This part is for loading photos cards into the "media" section -->

      <div style="margin-top: 1rem;">
        <div style="display: flex; align-items: flex-start; flex-direction: row;">

          <div style="width: 15vw;">
            <n-h2>Tags</n-h2>
            <n-collapse :default-expanded-names="['show']">

              <n-collapse-item title="Add New Tag" name="add">
                <n-card title="" size="small" >
                      Add a New Tag
                      <template #footer>
                        <n-space vertical>
                          <n-input v-model:value="addNewTagInput" type="text" placeholder="Tag Name" @keyup.enter="createTag" />
                          <n-button strong secondary style="width: 100%; " @click="createTag">
                            Add
                          </n-button>
                        </n-space>
                      </template>
                </n-card>
              </n-collapse-item>

              <n-collapse-item title="Tags" name="show">
                
                <n-flex vertical>
                  <n-tag v-for="tag in tagChildren" @click.left.native="selectTag(tag)"
                    :type="renderPhotoTagIds.includes(tag.tagId) ? 'success' : 'info'" @click.right.native="TagMenu(tag, $event)"
                    class="tagCard">
                      {{ tag.tagName > 15 ? tag.tagName.substring(0, 15) + "..." : tag.tagName }} 
                  </n-tag>
                </n-flex>
              </n-collapse-item>

            </n-collapse>
            
          </div>

          <MediaList style="margin-left: 2rem;" @queryPhotos="queryPhotos" @doDetach="removeFromTagSelectedPhoto" ref="mediaListRef" :nsOrNot="false" :detachEnabled="true" detachStr="Tag"/>

        </div>
        
      </div>
      <!-- ENDOF: This part is for loading photos cards into the "media" section -->

    </div>
    <!-- ENDOF: This part is for medias -->

    <!-- MENU PART -->

    <!-- This part is for redesign of right click menu for tag part -->
    <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="TagMenuOptions"
        :show="showTagMenu"
        :on-clickoutside="onClickOutsideTag"
        @select="handleSelectTag"
      />
    <!-- ENDOF: This part is for redesign of right click menu for tag part -->
    <!-- ENDOF: MENU PART -->

    <!-- MODAL PART -->

    <!-- This part is for modal of confirming to delete a tag -->
    <n-modal v-model:show="showDeleteTagConfirmModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE this tag?
        <p>*All photos associated with this tag will be detached (photos will not been deleted).</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteTag">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showDeleteTagConfirmModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete a tag -->

    <!-- This part is for modal of edit a tag's information -->
    <n-modal v-model:show="showEditTagModal">
      <n-card title="Edit Gallery" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>
                <n-input v-model:value="editTagInfo.tagName" type="text" placeholder="Tag Name" />
                <n-button strong secondary style="width: 100%; " @click="editTag">
                  Submit
                </n-button>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit a tag's information -->
    
    <!-- ENDOF: MODAL PART -->

    <n-back-top :right="100" />
  </div>

  
  
</template>

<script>

const mediaListRef = ref(null);

export default defineComponent({
  data() {
    return {
      renderPhotoTagIds: [],
      nsPathStr: "",
      userToken: localStorage.getItem("HRD-Token"),
      showPhotoSpaceMenu: false,
      showTagMenu: false,
      showPhotoSpaceMenuPhotoInfo: null,
      mouseX: null,
      mouseY: null,
      TagMenuOptions: [
        {
          label: "Edit",
          key: "edit",
          icon() {
            return h(NIcon, null, {
              default: () => h(Edit20Regular)
            });
          }
        },
        {
          label: "Delete",
          key: "delete",
          icon() {
            return h(NIcon, null, {
              default: () => h(Delete20Regular)
            });
          }
        },
      ],

      tagChildren: [],
      addNewTagInput: "",

      editTagInfo: null,

      showDeleteTagConfirmModal: false,
      showEditTagModal: false,
    }
  },
  setup() {
    
    return {
      
    };
  },
  methods: {
    selectTag(tag) {
      if (this.renderPhotoTagIds.includes(tag.tagId))
      {
        this.renderPhotoTagIds = this.renderPhotoTagIds.filter(tagIdT => tagIdT !== tag.tagId)
      }else {
        this.renderPhotoTagIds.push(tag.tagId)
      }
      if (this.renderPhotoTagIds.length > 0)
      {
        this.queryPhotos();
      }else {
        mediaListRef.value.updatePhotoList([]);
      }
    },
    removeFromTagSelectedPhoto () {
      if (this.renderPhotoTagIds.length === 0)
      {
        window.$message.warning("Cannot operate when no tags is selected!")
      }else if (this.renderPhotoTagIds.length > 1)
      {
        window.$message.warning("Cannot operate when multiple tags are selected!")
      }else {
          this.photoChildren.forEach((photo) => {
            if(photo["checked"])
            {
              axios({
                method: 'post',
                baseURL: '',
                url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteTag?photoId=" + photo.photoId + "&tagId=" + this.renderPhotoTagIds[0],
                headers: {
                  "HRD-Token": localStorage.getItem("HRD-Token")
                },
                data: {
              }
              }).then((response) => {
                if (response.data.msgCode === 200)
                {
                  this.queryPhotos();
                  window.$message.success("Tag detached!");
                }else {
                  window.$message.error("removeTag Response Error!")
                }
              })
              .catch(function (error) { // 请求失败处理
                // console.log(error);
                window.$message.error(error);
              });
            }
        })
        this.showPhotoMultiDetachModal = false;
        this.photoMultiSelectMode = false;
      }
      
    },
    blankSpaceMenu (e) {
      e.preventDefault();
      e.stopPropagation();
    },
    handleSelectTag(key) {
      this.showTagMenu = false;
      if (key === "edit")
      {
        this.showEditTagModal = true;
      }
      if (key === "delete")
      {
        this.showDeleteTagConfirmModal = true;
      }
    },
    onClickOutsideTag () {
      this.showTagMenu = false;
    },
    TagMenu(tag, e) {
      this.editTagInfo = {
        "tagId": tag.tagId,
        "tagName": tag.tagName
      };
      e.preventDefault();
      e.stopPropagation();
      this.mouseX = e.clientX;
      this.mouseY = e.clientY;
      this.showTagMenu = true;
    },
    deleteTag () {
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/tag/delete?tagId=" + this.editTagInfo.tagId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.getAllTags();
            this.photoChildren = []
            this.showDeleteTagConfirmModal = false;
            window.$message.success("Tag Deleted!");
            this.renderPhotoTagIds = this.renderPhotoTagIds.filter(tagId => tagId !== this.editTagInfo.tagId)
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    createTag() {
      if (this.addNewTagInput === null || this.addNewTagInput === "")
      {
        window.$message.warning("Please Input a Valid Tag Name!");
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/tag/insert",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "tagName": this.addNewTagInput,
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            window.$message.success("Created a New Tag!");
            this.getAllTags();
            this.addNewTagInput = "";
          }else {
            window.$message.error("createTag Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
    },
    editTag() {
      if (this.editTagInfo.tagName === null || this.editTagInfo.tagName === "")
      {
        window.$message.warning("Please Input a Valid Tag Name!");
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/tag/update",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: this.editTagInfo
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            window.$message.success("Information Updated!");
            this.getAllTags();
            this.showEditTagModal = false;
          }else {
            window.$message.error("Update Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
    },
    getAllTags() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/tag/getAll",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          let alltags = [];
          for(var i = 0; i < infoT.length; i ++)
          {
            var tag = infoT[i];
            alltags.push({
              "label": tag["tagName"], 
              "value": tag["tagId"]
            })
          }
          this.tagOptions = alltags;
          this.tagChildren = infoT;
        }else {
          window.$message.error("Tag Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    queryPhotos() {
      var queryTagStr = "";
      this.renderPhotoTagIds.forEach((tagId) => {queryTagStr += tagId + ","})
      queryTagStr = queryTagStr.substring(0, queryTagStr.length - 1)
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getByTags?tagIds=" + queryTagStr,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.photoChildren = infoT;
          mediaListRef.value.updatePhotoList(infoT);
        }else {
          window.$message.error("query photo Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.getAllTags();
  },
  components: {

  }
})

</script>

<style scoped>
.n-button {
  border-radius: 10px;
  width: 15rem;
  
}
.cardImgDiv {
  width: 255px;
  height: 128px;
}
.cardImg {
  object-fit: cover;
  width: 100%;
  max-width: 100%;
  height: 128px;
  overflow: hidden;
}
.n-card {
  /*cursor: pointer;*/
  padding: 0;
}
.tagCard {
  cursor: pointer;
  user-select: none;
}
</style>
